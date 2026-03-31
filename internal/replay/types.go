package replay

import "time"

// RequestMetadata represents request metadata for replay
type RequestMetadata struct {
	RequestID  string            `json:"request_id"`
	Timestamp  time.Time         `json:"timestamp"`
	UserID     string            `json:"user_id"`
	AppID      string            `json:"app_id"`
	ModelID    string            `json:"model_id"`
	Provider   string            `json:"provider"`
	Prompt     string            `json:"prompt"`
	Response   string            `json:"response"`
	TokenUsage TokenUsage        `json:"token_usage"`
	Latency    int64             `json:"latency_ms"`
	StatusCode int               `json:"status_code"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata"`
}

// TokenUsage represents token usage
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// HistoryFilter represents query filter for history
type HistoryFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	UserID    string
	ModelID   string
	Limit     int
}

// ReplayResult represents the result of a replay
type ReplayResult struct {
	RequestID  string            `json:"request_id"`
	Original   *RequestMetadata  `json:"original"`
	Replay     *RequestMetadata  `json:"replay"`
	Comparison *ComparisonResult `json:"comparison"`
	ReplayedAt time.Time         `json:"replayed_at"`
}

// ComparisonResult represents comparison between original and replay
type ComparisonResult struct {
	TokenDiff         ComparisonMetric `json:"token_diff"`
	LatencyDiff       ComparisonMetric `json:"latency_diff"`
	ContentSimilarity float64          `json:"content_similarity"`
}

// ComparisonMetric represents a comparison metric
type ComparisonMetric struct {
	Original int     `json:"original"`
	Replay   int     `json:"replay"`
	Diff     int     `json:"diff"`
	DiffPct  float64 `json:"diff_pct"`
}
