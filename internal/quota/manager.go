package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// QuotaManager manages token quotas
type QuotaManager struct {
	redis         *redis.Client
	logger        *zap.Logger
	deductScript  *redis.Script
	reserveScript *redis.Script
	settleScript  *redis.Script
}

// Lua script for atomic quota deduction
const deductQuotaScript = `
local user_key = KEYS[1]
local app_key = KEYS[2]
local model_key = KEYS[3]
local tokens = tonumber(ARGV[1])

-- Get current quotas
local user_data = redis.call('HMGET', user_key, 'total', 'used')
local app_data = redis.call('HMGET', app_key, 'total', 'used')
local model_data = redis.call('HMGET', model_key, 'total', 'used')

local user_total = tonumber(user_data[1]) or 0
local user_used = tonumber(user_data[2]) or 0
local app_total = tonumber(app_data[1]) or 0
local app_used = tonumber(app_data[2]) or 0
local model_total = tonumber(model_data[1]) or 0
local model_used = tonumber(model_data[2]) or 0

-- Check if sufficient quota
if user_total - user_used < tokens then
    return {err = "insufficient user quota"}
end
if app_total - app_used < tokens then
    return {err = "insufficient app quota"}
end
if model_total - model_used < tokens then
    return {err = "insufficient model quota"}
end

-- Deduct quota atomically
redis.call('HINCRBY', user_key, 'used', tokens)
redis.call('HINCRBY', app_key, 'used', tokens)
redis.call('HINCRBY', model_key, 'used', tokens)

return {ok = "success"}
`

// Lua script for atomic quota reservation
const reserveQuotaScript = `
local user_key = KEYS[1]
local app_key = KEYS[2]
local model_key = KEYS[3]
local tokens = tonumber(ARGV[1])

-- Get current quotas
local user_data = redis.call('HMGET', user_key, 'total', 'used', 'reserved')
local app_data = redis.call('HMGET', app_key, 'total', 'used', 'reserved')
local model_data = redis.call('HMGET', model_key, 'total', 'used', 'reserved')

local user_total = tonumber(user_data[1]) or 0
local user_used = tonumber(user_data[2]) or 0
local user_reserved = tonumber(user_data[3]) or 0
local app_total = tonumber(app_data[1]) or 0
local app_used = tonumber(app_data[2]) or 0
local app_reserved = tonumber(app_data[3]) or 0
local model_total = tonumber(model_data[1]) or 0
local model_used = tonumber(model_data[2]) or 0
local model_reserved = tonumber(model_data[3]) or 0

-- Check if sufficient quota
if user_total - user_used - user_reserved < tokens then
    return {err = "insufficient user quota"}
end
if app_total - app_used - app_reserved < tokens then
    return {err = "insufficient app quota"}
end
if model_total - model_used - model_reserved < tokens then
    return {err = "insufficient model quota"}
end

-- Reserve quota atomically
redis.call('HINCRBY', user_key, 'reserved', tokens)
redis.call('HINCRBY', app_key, 'reserved', tokens)
redis.call('HINCRBY', model_key, 'reserved', tokens)

return {ok = "success"}
`

// Lua script for atomic quota settlement
const settleQuotaScript = `
local user_key = KEYS[1]
local app_key = KEYS[2]
local model_key = KEYS[3]
local reserved_tokens = tonumber(ARGV[1])
local actual_tokens = tonumber(ARGV[2])

-- Calculate difference
local diff = actual_tokens - reserved_tokens

-- Update quotas atomically
redis.call('HINCRBY', user_key, 'reserved', -reserved_tokens)
redis.call('HINCRBY', app_key, 'reserved', -reserved_tokens)
redis.call('HINCRBY', model_key, 'reserved', -reserved_tokens)

redis.call('HINCRBY', user_key, 'used', actual_tokens)
redis.call('HINCRBY', app_key, 'used', actual_tokens)
redis.call('HINCRBY', model_key, 'used', actual_tokens)

return {ok = "success"}
`

// NewQuotaManager creates a new quota manager
func NewQuotaManager(redisClient *redis.Client, logger *zap.Logger) *QuotaManager {
	return &QuotaManager{
		redis:         redisClient,
		logger:        logger,
		deductScript:  redis.NewScript(deductQuotaScript),
		reserveScript: redis.NewScript(reserveQuotaScript),
		settleScript:  redis.NewScript(settleQuotaScript),
	}
}

// GetQuota retrieves a quota node
func (qm *QuotaManager) GetQuota(ctx context.Context, level QuotaLevel, identifier string) (*QuotaNode, error) {
	key := qm.buildKey(level, identifier)
	data, err := qm.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("quota not found: %s", identifier)
		}
		return nil, err
	}

	var node QuotaNode
	if err := json.Unmarshal([]byte(data), &node); err != nil {
		return nil, err
	}

	return &node, nil
}

// SetQuota sets a quota node
func (qm *QuotaManager) SetQuota(ctx context.Context, node *QuotaNode) error {
	node.UpdatedAt = time.Now()
	key := qm.buildKey(node.Level, node.Identifier)

	data, err := json.Marshal(node)
	if err != nil {
		return err
	}

	return qm.redis.Set(ctx, key, data, 0).Err()
}

// ReserveQuota reserves quota for a streaming request using Lua script for atomicity
func (qm *QuotaManager) ReserveQuota(ctx context.Context, userID, appID, modelID string, estimatedTokens int64) (string, error) {
	reservationID := uuid.New().String()

	userKey := qm.buildKey(QuotaLevelUser, userID)
	appKey := qm.buildKey(QuotaLevelApp, appID)
	modelKey := qm.buildKey(QuotaLevelModel, modelID)

	result, err := qm.reserveScript.Run(ctx, qm.redis,
		[]string{userKey, appKey, modelKey},
		estimatedTokens,
	).Result()

	if err != nil {
		qm.logger.Error("quota reservation failed", zap.Error(err))
		return "", err
	}

	// Check result
	if resultMap, ok := result.(map[interface{}]interface{}); ok {
		if errMsg, exists := resultMap["err"]; exists {
			return "", fmt.Errorf("%v", errMsg)
		}
	}

	// Record reservation
	reservation := &QuotaReservation{
		ID:             reservationID,
		UserID:         userID,
		AppID:          appID,
		ModelID:        modelID,
		ReservedTokens: estimatedTokens,
		ActualTokens:   0,
		Status:         ReservationStatusPending,
		CreatedAt:      time.Now(),
	}

	data, _ := json.Marshal(reservation)
	qm.redis.Set(ctx, fmt.Sprintf("reservation:%s", reservationID), data, 10*time.Minute)

	qm.logger.Info("quota reserved",
		zap.String("reservation_id", reservationID),
		zap.Int64("tokens", estimatedTokens))

	return reservationID, nil
}

// SettleQuota settles quota after streaming request completes using Lua script for atomicity
func (qm *QuotaManager) SettleQuota(ctx context.Context, reservationID string, actualTokens int64) error {
	// Get reservation
	data, err := qm.redis.Get(ctx, fmt.Sprintf("reservation:%s", reservationID)).Result()
	if err != nil {
		return fmt.Errorf("reservation not found: %s", reservationID)
	}

	var reservation QuotaReservation
	if err := json.Unmarshal([]byte(data), &reservation); err != nil {
		return err
	}

	userKey := qm.buildKey(QuotaLevelUser, reservation.UserID)
	appKey := qm.buildKey(QuotaLevelApp, reservation.AppID)
	modelKey := qm.buildKey(QuotaLevelModel, reservation.ModelID)

	result, err := qm.settleScript.Run(ctx, qm.redis,
		[]string{userKey, appKey, modelKey},
		reservation.ReservedTokens,
		actualTokens,
	).Result()

	if err != nil {
		qm.logger.Error("quota settlement failed", zap.Error(err))
		return err
	}

	// Check result
	if resultMap, ok := result.(map[interface{}]interface{}); ok {
		if errMsg, exists := resultMap["err"]; exists {
			return fmt.Errorf("%v", errMsg)
		}
	}

	// Update reservation status
	reservation.Status = ReservationStatusSettled
	reservation.ActualTokens = actualTokens
	now := time.Now()
	reservation.SettledAt = &now

	reservationData, _ := json.Marshal(reservation)
	qm.redis.Set(ctx, fmt.Sprintf("reservation:%s", reservationID), reservationData, 10*time.Minute)

	diff := actualTokens - reservation.ReservedTokens
	qm.logger.Info("quota settled",
		zap.String("reservation_id", reservationID),
		zap.Int64("actual_tokens", actualTokens),
		zap.Int64("diff", diff))

	return nil
}

// DeductQuota deducts quota for non-streaming requests using Lua script for atomicity
func (qm *QuotaManager) DeductQuota(ctx context.Context, userID, appID, modelID string, tokens int64) error {
	userKey := qm.buildKey(QuotaLevelUser, userID)
	appKey := qm.buildKey(QuotaLevelApp, appID)
	modelKey := qm.buildKey(QuotaLevelModel, modelID)

	result, err := qm.deductScript.Run(ctx, qm.redis,
		[]string{userKey, appKey, modelKey},
		tokens,
	).Result()

	if err != nil {
		qm.logger.Error("quota deduction failed", zap.Error(err))
		return err
	}

	// Check result
	if resultMap, ok := result.(map[interface{}]interface{}); ok {
		if errMsg, exists := resultMap["err"]; exists {
			return fmt.Errorf("%v", errMsg)
		}
	}

	qm.logger.Info("quota deducted",
		zap.String("user_id", userID),
		zap.String("app_id", appID),
		zap.String("model_id", modelID),
		zap.Int64("tokens", tokens))

	return nil
}

// buildKey builds the Redis key for a quota node
func (qm *QuotaManager) buildKey(level QuotaLevel, identifier string) string {
	return fmt.Sprintf("quota:%s:%s", level, identifier)
}
