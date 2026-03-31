package handler

import (
	"fmt"
	"net/http"
	"time"

	"ai-ide-gateway/internal/replay"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ReplayHandler handles traffic replay API endpoints
type ReplayHandler struct {
	service *replay.ReplayService
	logger  *zap.Logger
}

// NewReplayHandler creates a new ReplayHandler
func NewReplayHandler(service *replay.ReplayService, logger *zap.Logger) *ReplayHandler {
	return &ReplayHandler{
		service: service,
		logger:  logger,
	}
}

// ListHistory handles GET /v1/replay/history
// Query params: start_time, end_time (RFC3339), user_id, model_id, limit (int)
func (h *ReplayHandler) ListHistory(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	records, err := h.service.FetchHistory(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("fetch history failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"count":   len(records),
	})
}

// StartReplay handles POST /v1/replay/run
func (h *ReplayHandler) StartReplay(c *gin.Context) {
	var req replay.ReplayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.GatewayURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gateway_url is required"})
		return
	}
	if req.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
		return
	}

	results, err := h.service.Replay(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("replay failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "replay failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// parseFilter extracts HistoryFilter from query parameters
func parseFilter(c *gin.Context) (replay.HistoryFilter, error) {
	var filter replay.HistoryFilter

	if v := c.Query("start_time"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, fmt.Errorf("invalid start_time: %v", err)
		}
		filter.StartTime = &t
	}
	if v := c.Query("end_time"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, fmt.Errorf("invalid end_time: %v", err)
		}
		filter.EndTime = &t
	}
	filter.UserID = c.Query("user_id")
	filter.ModelID = c.Query("model_id")

	if v := c.Query("limit"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			filter.Limit = n
		}
	}

	return filter, nil
}
