package quota

import "time"

// QuotaLevel represents the level in the quota tree
type QuotaLevel string

const (
	QuotaLevelUser  QuotaLevel = "user"
	QuotaLevelApp   QuotaLevel = "app"
	QuotaLevelModel QuotaLevel = "model"
)

// QuotaNode represents a node in the quota tree
type QuotaNode struct {
	Level      QuotaLevel `json:"level"`
	Identifier string     `json:"identifier"`
	Total      int64      `json:"total"`
	Used       int64      `json:"used"`
	Reserved   int64      `json:"reserved"`
	Children   []string   `json:"children"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// QuotaReservation represents a quota reservation
type QuotaReservation struct {
	ID             string            `json:"id"`
	UserID         string            `json:"user_id"`
	AppID          string            `json:"app_id"`
	ModelID        string            `json:"model_id"`
	ReservedTokens int64             `json:"reserved_tokens"`
	ActualTokens   int64             `json:"actual_tokens"`
	Status         ReservationStatus `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	SettledAt      *time.Time        `json:"settled_at,omitempty"`
}

// ReservationStatus represents the status of a reservation
type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "pending"
	ReservationStatusSettled   ReservationStatus = "settled"
	ReservationStatusCancelled ReservationStatus = "cancelled"
)

// TokenUsage represents token usage information
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
