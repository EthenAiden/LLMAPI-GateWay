package auth

import "time"

// APIKey represents an API key
type APIKey struct {
	ID        string    `json:"id"`
	KeyHash   string    `json:"key_hash"`
	UserID    string    `json:"user_id"`
	AppID     string    `json:"app_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthContext represents authentication context
type AuthContext struct {
	APIKey string
	UserID string
	AppID  string
}

// AuthError represents an authentication error
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
