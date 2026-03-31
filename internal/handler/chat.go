package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ai-ide-gateway/internal/adapter"
	"ai-ide-gateway/internal/auth"
	"ai-ide-gateway/internal/breaker"
	"ai-ide-gateway/internal/monitor"
	"ai-ide-gateway/internal/quota"
	"ai-ide-gateway/internal/replay"
	"ai-ide-gateway/internal/router"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ChatHandler ???? /v1/chat/completions ???????? Handler
type ChatHandler struct {
	authenticator   *auth.Authenticator
	routeManager    *router.RouteManager
	failover        *router.FailoverExecutor
	adapterFactory  *adapter.AdapterFactory
	rateLimiter     *breaker.RateLimiter
	quotaManager    *quota.QuotaManager
	replayCollector *replay.ReplayCollector
	metrics         *monitor.Metrics
	httpClient      *http.Client
	logger          *zap.Logger
}

// NewChatHandler ???? ChatHandler ????????????????
func NewChatHandler(
	authenticator *auth.Authenticator,
	routeManager *router.RouteManager,
	failover *router.FailoverExecutor,
	adapterFactory *adapter.AdapterFactory,
	rateLimiter *breaker.RateLimiter,
	quotaManager *quota.QuotaManager,
	replayCollector *replay.ReplayCollector,
	metrics *monitor.Metrics,
	logger *zap.Logger,
) *ChatHandler {
	return &ChatHandler{
		authenticator:   authenticator,
		routeManager:    routeManager,
		failover:        failover,
		adapterFactory:  adapterFactory,
		rateLimiter:     rateLimiter,
		quotaManager:    quotaManager,
		replayCollector: replayCollector,
		metrics:         metrics,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		logger: logger,
	}
}

// Handle ??????????????????? OpenAI ????? /v1/chat/completions ???
func (h *ChatHandler) Handle(c *gin.Context) {
	startTime := time.Now()
	requestID := uuid.New().String()

	// 1. ??????? Authorization Header ???????? API Key
	authCtx, err := h.authenticator.Authenticate(c.Request.Context(), c.GetHeader("Authorization"))
	if err != nil {
		h.logger.Warn("authentication failed", zap.String("request_id", requestID), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 2. ???? OpenAI ?????????
	var req adapter.OpenAIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 3. ??????ï“???/???/????????
	if err := h.rateLimiter.CheckMultiDimension(c.Request.Context(),
		authCtx.UserID, authCtx.AppID, req.Model, 1); err != nil {
		h.metrics.RateLimitExceeded.Inc()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}

	// 4. ???¡¤???????????? Provider ????
	rule, err := h.routeManager.GetRule(req.Model)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("model not found: %s", req.Model)})
		return
	}

	// 5. ???§¿??????????router.ProviderType ?? adapter.ProviderType ?????????
	adpt, err := h.adapterFactory.GetAdapter(adapter.ProviderType(rule.Provider))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "adapter not found"})
		return
	}

	// 6. ????????? vs ?????
	if req.Stream {
		h.handleStream(c, requestID, startTime, authCtx, &req, adpt, rule)
	} else {
		h.handleNonStream(c, requestID, startTime, authCtx, &req, adpt, rule)
	}
}

// handleNonStream ???????????????? ?? ??? ?? ????? OpenAI ??? ?? ?????? ?? ??????????
func (h *ChatHandler) handleNonStream(
	c *gin.Context,
	requestID string,
	startTime time.Time,
	authCtx *auth.AuthContext,
	req *adapter.OpenAIRequest,
	adpt adapter.ProtocolAdapter,
	rule *router.RouteRule,
) {
	var openaiResp *adapter.OpenAIResponse

	// ??? Failover ?????????? API Key???????????????§Ý?
	err := h.failover.ExecuteWithFailover(c.Request.Context(), req.Model, func(apiKey string) error {
		// ?????????
		upstreamReq, err := adpt.ConvertRequest(c.Request.Context(), req)
		if err != nil {
			return fmt.Errorf("request conversion failed: %w", err)
		}

		// ???? HTTP ??????? API Key
		httpReq, err := buildHTTPRequest(c.Request.Context(), upstreamReq, apiKey)
		if err != nil {
			return err
		}

		// ????????????
		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("upstream request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return fmt.Errorf("upstream error: status %d", resp.StatusCode)
		}

		// ????????
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		// ???? OpenAI ??????
		openaiResp, err = adpt.ConvertResponse(c.Request.Context(), &adapter.UpstreamResponse{
			StatusCode: resp.StatusCode,
			Body:       body,
		})
		return err
	})

	if err != nil {
		h.metrics.RequestErrors.Inc()
		h.logger.Error("request failed after all failovers", zap.String("request_id", requestID), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream unavailable"})
		return
	}

	// 7. ??? Token ???????????????
	if openaiResp != nil {
		totalTokens := int64(openaiResp.Usage.TotalTokens)
		if err := h.quotaManager.DeductQuota(c.Request.Context(),
			authCtx.UserID, authCtx.AppID, req.Model, totalTokens); err != nil {
			h.logger.Warn("quota deduction failed", zap.Error(err))
		}
	}

	// 8. ???????????????¡¤??????? IO??
	if h.replayCollector != nil && openaiResp != nil {
		h.replayCollector.Collect(c.Request.Context(), &replay.RequestMetadata{
			RequestID:  requestID,
			Timestamp:  startTime,
			UserID:     authCtx.UserID,
			AppID:      authCtx.AppID,
			ModelID:    req.Model,
			Provider:   string(adpt.GetProviderType()),
			Latency:    time.Since(startTime).Milliseconds(),
			StatusCode: http.StatusOK,
			TokenUsage: replay.TokenUsage{
				PromptTokens:     openaiResp.Usage.PromptTokens,
				CompletionTokens: openaiResp.Usage.CompletionTokens,
				TotalTokens:      openaiResp.Usage.TotalTokens,
			},
		})
	}

	h.metrics.RequestTotal.Inc()
	c.JSON(http.StatusOK, openaiResp)
}

// handleStream ????????????????? ?? ??? SSE ?? ??????? Token ?? ??????
func (h *ChatHandler) handleStream(
	c *gin.Context,
	requestID string,
	startTime time.Time,
	authCtx *auth.AuthContext,
	req *adapter.OpenAIRequest,
	adpt adapter.ProtocolAdapter,
	rule *router.RouteRule,
) {
	ctx := c.Request.Context()

	// ??????estimatedTokens = 0 ??????¦Ë?????????????§µ????
	reservationID, err := h.quotaManager.ReserveQuota(ctx, authCtx.UserID, authCtx.AppID, req.Model, 0)
	if err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "quota reservation failed: " + err.Error()})
		return
	}

	var (
		finalPromptTokens     int
		finalCompletionTokens int
	)

	// ???? SSE Headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	streamErr := h.failover.ExecuteWithFailover(ctx, req.Model, func(apiKey string) error {
		upstreamReq, err := adpt.ConvertRequest(ctx, req)
		if err != nil {
			return err
		}

		httpReq, err := buildHTTPRequest(ctx, upstreamReq, apiKey)
		if err != nil {
			return err
		}

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// ???§Ø??????? SSE ???????????? Token
		pt, ct, sErr := streamAndCountTokens(c.Writer, flusher, resp.Body, adpt, ctx)
		finalPromptTokens = pt
		finalCompletionTokens = ct
		return sErr
	})

	// ??????/??????????? Token ?????????
	actualTokens := int64(finalPromptTokens + finalCompletionTokens)
	if settleErr := h.quotaManager.SettleQuota(ctx, reservationID, actualTokens); settleErr != nil {
		h.logger.Warn("quota settle failed", zap.String("reservation_id", reservationID), zap.Error(settleErr))
	}

	// ?????????????
	if h.replayCollector != nil {
		h.replayCollector.Collect(ctx, &replay.RequestMetadata{
			RequestID:  requestID,
			Timestamp:  startTime,
			UserID:     authCtx.UserID,
			AppID:      authCtx.AppID,
			ModelID:    req.Model,
			Provider:   string(adpt.GetProviderType()),
			Latency:    time.Since(startTime).Milliseconds(),
			StatusCode: http.StatusOK,
			TokenUsage: replay.TokenUsage{
				PromptTokens:     finalPromptTokens,
				CompletionTokens: finalCompletionTokens,
				TotalTokens:      int(actualTokens),
			},
		})
	}

	if streamErr != nil {
		h.logger.Error("stream failed", zap.String("request_id", requestID), zap.Error(streamErr))
	}
	h.metrics.RequestTotal.Inc()
}

// buildHTTPRequest ???? UpstreamRequest ???? *http.Request??????? API Key
func buildHTTPRequest(ctx context.Context, upstreamReq *adapter.UpstreamRequest, apiKey string) (*http.Request, error) {
	bodyBytes, err := json.Marshal(upstreamReq.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, upstreamReq.Method, upstreamReq.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	for k, v := range upstreamReq.Headers {
		httpReq.Header.Set(k, v)
	}

	// ????????¦Ä???¨¹?????????? API Key??Cursor/Antigravity ??? Bearer Token??
	if httpReq.Header.Get("Authorization") == "" && apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	return httpReq, nil
}

// streamAndCountTokens ????????????? Token ?????
func streamAndCountTokens(w http.ResponseWriter, flusher http.Flusher, body io.ReadCloser, adpt adapter.ProtocolAdapter, ctx context.Context) (int, int, error) {
	var promptTokens, completionTokens int
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return promptTokens, completionTokens, ctx.Err()
		default:
		}

		line := scanner.Bytes()

		// SSE ?????data: {...}
		if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))

			// ??????????????
			if bytes.Equal(data, []byte("[DONE]")) {
				if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
					return promptTokens, completionTokens, err
				}
				flusher.Flush()
				break
			}

			// ???? JSON ?????
			var chunk adapter.OpenAIStreamChunk
			if err := json.Unmarshal(data, &chunk); err == nil {
				// ??? completion tokens????? delta ??????? 1 token???????
				for _, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						// ????????????/4 ???? token ??
						tokens := len(choice.Delta.Content) / 4
						if tokens == 0 {
							tokens = 1
						}
						completionTokens += tokens
					}
				}
			}

			// ?????????
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return promptTokens, completionTokens, err
			}
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return promptTokens, completionTokens, err
	}

	// ????? prompt tokens????????????? chunk ?? usage ??¦Ë????
	// ????????????????????????????????????§ß???
	if promptTokens == 0 {
		promptTokens = 100 // ???????
	}

	return promptTokens, completionTokens, nil
}
