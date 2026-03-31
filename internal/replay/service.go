package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// ReplayService reads historical records from Kafka and re-executes them
type ReplayService struct {
	brokers    []string
	topic      string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewReplayService creates a new replay service
func NewReplayService(brokers []string, topic string, logger *zap.Logger) *ReplayService {
	return &ReplayService{
		brokers: brokers,
		topic:   topic,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
		logger: logger,
	}
}

// FetchHistory reads RequestMetadata records from Kafka that match the filter.
// It reads from the beginning of the topic up to the current end, respecting the limit.
func (s *ReplayService) FetchHistory(ctx context.Context, filter HistoryFilter) ([]*RequestMetadata, error) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     s.brokers,
		Topic:       s.topic,
		GroupID:     "", // no group — start from the beginning each time
		StartOffset: kafka.FirstOffset,
		MaxBytes:    10 << 20, // 10 MB
	})
	defer r.Close()

	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	var results []*RequestMetadata
	deadline := time.Now().Add(10 * time.Second) // max time to spend reading

	for len(results) < limit {
		readCtx, cancel := context.WithDeadline(ctx, deadline)
		msg, err := r.ReadMessage(readCtx)
		cancel()
		if err != nil {
			// Deadline or EOF — we've read all available messages
			break
		}

		var meta RequestMetadata
		if err := json.Unmarshal(msg.Value, &meta); err != nil {
			s.logger.Warn("failed to unmarshal replay record", zap.Error(err))
			continue
		}

		if !matchesFilter(&meta, filter) {
			continue
		}

		results = append(results, &meta)
	}

	return results, nil
}

// matchesFilter returns true if the metadata passes all active filter criteria
func matchesFilter(meta *RequestMetadata, f HistoryFilter) bool {
	if f.StartTime != nil && meta.Timestamp.Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && meta.Timestamp.After(*f.EndTime) {
		return false
	}
	if f.UserID != "" && meta.UserID != f.UserID {
		return false
	}
	if f.ModelID != "" && meta.ModelID != f.ModelID {
		return false
	}
	return true
}

// ReplayRequest holds the parameters for a replay operation
type ReplayRequest struct {
	// Filter selects which historical records to replay
	Filter HistoryFilter `json:"filter"`
	// TargetModel overrides the model used in replayed requests (for migration comparison)
	TargetModel string `json:"target_model,omitempty"`
	// GatewayURL is the gateway endpoint to send replayed requests to
	GatewayURL string `json:"gateway_url"`
	// APIKey used for replayed requests
	APIKey string `json:"api_key"`
}

// Replay fetches historical records and re-executes each request, returning comparison results
func (s *ReplayService) Replay(ctx context.Context, req ReplayRequest) ([]*ReplayResult, error) {
	records, err := s.FetchHistory(ctx, req.Filter)
	if err != nil {
		return nil, fmt.Errorf("fetch history: %w", err)
	}

	results := make([]*ReplayResult, 0, len(records))
	for _, original := range records {
		result, err := s.replayOne(ctx, original, req)
		if err != nil {
			s.logger.Warn("replay failed for request",
				zap.String("request_id", original.RequestID),
				zap.Error(err))
			// Include failed result so caller knows about it
			results = append(results, &ReplayResult{
				RequestID:  original.RequestID,
				Original:   original,
				ReplayedAt: time.Now(),
				Replay: &RequestMetadata{
					Error: err.Error(),
				},
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// replayOne re-executes a single historical request and compares results
func (s *ReplayService) replayOne(ctx context.Context, original *RequestMetadata, req ReplayRequest) (*ReplayResult, error) {
	// Reconstruct an OpenAI-compatible request body
	model := original.ModelID
	if req.TargetModel != "" {
		model = req.TargetModel
	}

	// original.Prompt holds the serialized messages JSON (set by collector)
	// If it's empty, we can only send a minimal request
	var messages interface{}
	if original.Prompt != "" {
		var msgs []map[string]string
		if err := json.Unmarshal([]byte(original.Prompt), &msgs); err == nil {
			messages = msgs
		} else {
			messages = []map[string]string{{"role": "user", "content": original.Prompt}}
		}
	} else {
		messages = []map[string]string{{"role": "user", "content": "(replay: original prompt not captured)"}}
	}

	payload := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	gatewayURL := strings.TrimRight(req.GatewayURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, gatewayURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	start := time.Now()
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("replay request failed: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(start).Milliseconds()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read replay response: %w", err)
	}

	replayMeta := &RequestMetadata{
		RequestID:  original.RequestID + "_replay",
		Timestamp:  time.Now(),
		UserID:     original.UserID,
		AppID:      original.AppID,
		ModelID:    model,
		Latency:    latency,
		StatusCode: resp.StatusCode,
	}

	// Parse usage from response
	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &openaiResp); err == nil {
		replayMeta.TokenUsage = TokenUsage{
			PromptTokens:     openaiResp.Usage.PromptTokens,
			CompletionTokens: openaiResp.Usage.CompletionTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		}
		if len(openaiResp.Choices) > 0 {
			replayMeta.Response = openaiResp.Choices[0].Message.Content
		}
	} else {
		replayMeta.Error = fmt.Sprintf("upstream status %d", resp.StatusCode)
	}

	comparison := compare(original, replayMeta)

	return &ReplayResult{
		RequestID:  original.RequestID,
		Original:   original,
		Replay:     replayMeta,
		Comparison: comparison,
		ReplayedAt: time.Now(),
	}, nil
}

// compare builds a ComparisonResult between original and replay metadata
func compare(orig, replay *RequestMetadata) *ComparisonResult {
	tokenDiff := orig.TokenUsage.TotalTokens - replay.TokenUsage.TotalTokens
	tokenDiffPct := 0.0
	if orig.TokenUsage.TotalTokens > 0 {
		tokenDiffPct = float64(tokenDiff) / float64(orig.TokenUsage.TotalTokens) * 100
	}

	latencyDiff := int(orig.Latency) - int(replay.Latency)
	latencyDiffPct := 0.0
	if orig.Latency > 0 {
		latencyDiffPct = float64(latencyDiff) / float64(orig.Latency) * 100
	}

	similarity := contentSimilarity(orig.Response, replay.Response)

	return &ComparisonResult{
		TokenDiff: ComparisonMetric{
			Original: orig.TokenUsage.TotalTokens,
			Replay:   replay.TokenUsage.TotalTokens,
			Diff:     tokenDiff,
			DiffPct:  tokenDiffPct,
		},
		LatencyDiff: ComparisonMetric{
			Original: int(orig.Latency),
			Replay:   int(replay.Latency),
			Diff:     latencyDiff,
			DiffPct:  latencyDiffPct,
		},
		ContentSimilarity: similarity,
	}
}

// contentSimilarity returns a rough character-overlap similarity score (0.0–1.0)
func contentSimilarity(a, b string) float64 {
	if a == "" && b == "" {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}
	// Jaccard similarity over rune trigrams
	setA := trigrams(a)
	setB := trigrams(b)
	intersection := 0
	for t := range setA {
		if setB[t] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

func trigrams(s string) map[string]bool {
	runes := []rune(s)
	set := make(map[string]bool)
	for i := 0; i+3 <= len(runes); i++ {
		t := string(runes[i : i+3])
		if utf8.ValidString(t) {
			set[t] = true
		}
	}
	return set
}
