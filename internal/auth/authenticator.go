package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// Authenticator handles API key authentication
type Authenticator struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(redisClient *redis.Client, logger *zap.Logger) *Authenticator {
	return &Authenticator{
		redis:  redisClient,
		logger: logger,
	}
}

// Authenticate authenticates a request using API key
func (a *Authenticator) Authenticate(ctx context.Context, authHeader string) (*AuthContext, error) {
	if authHeader == "" {
		return nil, &AuthError{
			Code:    "missing_auth",
			Message: "Missing Authorization header",
		}
	}

	// Parse Bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, &AuthError{
			Code:    "invalid_auth_format",
			Message: "Invalid Authorization header format",
		}
	}

	apiKey := parts[1]

	// Verify API key
	keyHash := hashAPIKey(apiKey)
	data, err := a.redis.Get(ctx, fmt.Sprintf("apikey:%s", keyHash)).Result()
	if err != nil {
		if err == redis.Nil {
			a.logger.Warn("invalid API key", zap.String("key_hash", keyHash[:8]))
			return nil, &AuthError{
				Code:    "invalid_api_key",
				Message: "Invalid API key",
			}
		}
		return nil, err
	}

	// Parse API key data
	var apiKeyData APIKey
	if err := parseAPIKeyData(data, &apiKeyData); err != nil {
		return nil, err
	}

	// Check if key is active
	if apiKeyData.Status != "active" {
		return nil, &AuthError{
			Code:    "inactive_api_key",
			Message: "API key is not active",
		}
	}

	a.logger.Debug("API key authenticated",
		zap.String("user_id", apiKeyData.UserID),
		zap.String("app_id", apiKeyData.AppID))

	return &AuthContext{
		APIKey: apiKey,
		UserID: apiKeyData.UserID,
		AppID:  apiKeyData.AppID,
	}, nil
}

// CreateAPIKey creates a new API key
func (a *Authenticator) CreateAPIKey(ctx context.Context, userID, appID string) (string, error) {
	// Generate API key
	apiKey := generateAPIKey()
	keyHash := hashAPIKey(apiKey)

	// Store in Redis
	apiKeyData := APIKey{
		ID:      keyHash,
		KeyHash: keyHash,
		UserID:  userID,
		AppID:   appID,
		Status:  "active",
	}

	data := fmt.Sprintf(`{"id":"%s","key_hash":"%s","user_id":"%s","app_id":"%s","status":"active"}`,
		apiKeyData.ID, apiKeyData.KeyHash, apiKeyData.UserID, apiKeyData.AppID)

	if err := a.redis.Set(ctx, fmt.Sprintf("apikey:%s", keyHash), data, 0).Err(); err != nil {
		return "", err
	}

	a.logger.Info("API key created",
		zap.String("user_id", userID),
		zap.String("app_id", appID))

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (a *Authenticator) RevokeAPIKey(ctx context.Context, apiKey string) error {
	keyHash := hashAPIKey(apiKey)
	return a.redis.Del(ctx, fmt.Sprintf("apikey:%s", keyHash)).Err()
}

// hashAPIKey hashes an API key
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// generateAPIKey generates a new cryptographically random API key
func generateAPIKey() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random API key: %v", err))
	}
	return "sk-" + hex.EncodeToString(b)
}

// parseAPIKeyData parses API key data from JSON string
func parseAPIKeyData(data string, apiKey *APIKey) error {
	return json.Unmarshal([]byte(data), apiKey)
}
