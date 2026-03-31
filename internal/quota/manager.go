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
	redis  *redis.Client
	logger *zap.Logger
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager(redisClient *redis.Client, logger *zap.Logger) *QuotaManager {
	return &QuotaManager{
		redis:  redisClient,
		logger: logger,
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

// ReserveQuota reserves quota for a streaming request
func (qm *QuotaManager) ReserveQuota(ctx context.Context, userID, appID, modelID string, estimatedTokens int64) (string, error) {
	reservationID := uuid.New().String()

	// Check if quotas exist and have sufficient tokens
	userQuota, err := qm.GetQuota(ctx, QuotaLevelUser, userID)
	if err != nil {
		return "", fmt.Errorf("user quota not found: %v", err)
	}

	appQuota, err := qm.GetQuota(ctx, QuotaLevelApp, appID)
	if err != nil {
		return "", fmt.Errorf("app quota not found: %v", err)
	}

	modelQuota, err := qm.GetQuota(ctx, QuotaLevelModel, modelID)
	if err != nil {
		return "", fmt.Errorf("model quota not found: %v", err)
	}

	// Check if sufficient quota available
	if userQuota.Total-userQuota.Used-userQuota.Reserved < estimatedTokens {
		return "", fmt.Errorf("insufficient user quota")
	}

	if appQuota.Total-appQuota.Used-appQuota.Reserved < estimatedTokens {
		return "", fmt.Errorf("insufficient app quota")
	}

	if modelQuota.Total-modelQuota.Used-modelQuota.Reserved < estimatedTokens {
		return "", fmt.Errorf("insufficient model quota")
	}

	// Reserve quota
	userQuota.Reserved += estimatedTokens
	appQuota.Reserved += estimatedTokens
	modelQuota.Reserved += estimatedTokens

	if err := qm.SetQuota(ctx, userQuota); err != nil {
		return "", err
	}

	if err := qm.SetQuota(ctx, appQuota); err != nil {
		return "", err
	}

	if err := qm.SetQuota(ctx, modelQuota); err != nil {
		return "", err
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

// SettleQuota settles quota after streaming request completes
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

	// Get quotas
	userQuota, _ := qm.GetQuota(ctx, QuotaLevelUser, reservation.UserID)
	appQuota, _ := qm.GetQuota(ctx, QuotaLevelApp, reservation.AppID)
	modelQuota, _ := qm.GetQuota(ctx, QuotaLevelModel, reservation.ModelID)

	// Calculate difference
	diff := actualTokens - reservation.ReservedTokens

	if diff > 0 {
		// Need to deduct additional tokens
		if err := qm.deductAdditional(ctx, userQuota, appQuota, modelQuota, diff); err != nil {
			return err
		}
	} else if diff < 0 {
		// Need to refund tokens
		if err := qm.refund(ctx, userQuota, appQuota, modelQuota, -diff); err != nil {
			return err
		}
	}

	// Update reservation status
	reservation.Status = ReservationStatusSettled
	reservation.ActualTokens = actualTokens
	now := time.Now()
	reservation.SettledAt = &now

	reservationData, _ := json.Marshal(reservation)
	qm.redis.Set(ctx, fmt.Sprintf("reservation:%s", reservationID), reservationData, 10*time.Minute)

	qm.logger.Info("quota settled",
		zap.String("reservation_id", reservationID),
		zap.Int64("actual_tokens", actualTokens),
		zap.Int64("diff", diff))

	return nil
}

// DeductQuota deducts quota for non-streaming requests
func (qm *QuotaManager) DeductQuota(ctx context.Context, userID, appID, modelID string, tokens int64) error {
	userQuota, err := qm.GetQuota(ctx, QuotaLevelUser, userID)
	if err != nil {
		return err
	}

	appQuota, err := qm.GetQuota(ctx, QuotaLevelApp, appID)
	if err != nil {
		return err
	}

	modelQuota, err := qm.GetQuota(ctx, QuotaLevelModel, modelID)
	if err != nil {
		return err
	}

	// Check if sufficient quota
	if userQuota.Total-userQuota.Used < tokens {
		return fmt.Errorf("insufficient user quota")
	}

	if appQuota.Total-appQuota.Used < tokens {
		return fmt.Errorf("insufficient app quota")
	}

	if modelQuota.Total-modelQuota.Used < tokens {
		return fmt.Errorf("insufficient model quota")
	}

	// Deduct quota
	userQuota.Used += tokens
	appQuota.Used += tokens
	modelQuota.Used += tokens

	if err := qm.SetQuota(ctx, userQuota); err != nil {
		return err
	}

	if err := qm.SetQuota(ctx, appQuota); err != nil {
		return err
	}

	if err := qm.SetQuota(ctx, modelQuota); err != nil {
		return err
	}

	qm.logger.Info("quota deducted",
		zap.String("user_id", userID),
		zap.String("app_id", appID),
		zap.String("model_id", modelID),
		zap.Int64("tokens", tokens))

	return nil
}

// deductAdditional deducts additional tokens
func (qm *QuotaManager) deductAdditional(ctx context.Context, userQuota, appQuota, modelQuota *QuotaNode, tokens int64) error {
	userQuota.Reserved -= tokens
	userQuota.Used += tokens
	appQuota.Reserved -= tokens
	appQuota.Used += tokens
	modelQuota.Reserved -= tokens
	modelQuota.Used += tokens

	if err := qm.SetQuota(ctx, userQuota); err != nil {
		return err
	}

	if err := qm.SetQuota(ctx, appQuota); err != nil {
		return err
	}

	return qm.SetQuota(ctx, modelQuota)
}

// refund refunds tokens
func (qm *QuotaManager) refund(ctx context.Context, userQuota, appQuota, modelQuota *QuotaNode, tokens int64) error {
	userQuota.Reserved -= tokens
	appQuota.Reserved -= tokens
	modelQuota.Reserved -= tokens

	if err := qm.SetQuota(ctx, userQuota); err != nil {
		return err
	}

	if err := qm.SetQuota(ctx, appQuota); err != nil {
		return err
	}

	return qm.SetQuota(ctx, modelQuota)
}

// buildKey builds the Redis key for a quota node
func (qm *QuotaManager) buildKey(level QuotaLevel, identifier string) string {
	return fmt.Sprintf("quota:%s:%s", level, identifier)
}
