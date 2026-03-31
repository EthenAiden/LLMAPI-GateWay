package router

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// FailoverExecutor handles failover logic across multiple levels
type FailoverExecutor struct {
	routeManager   *RouteManager
	circuitBreaker CircuitBreaker
	logger         *zap.Logger
	httpClient     *http.Client
}

// CircuitBreaker interface for circuit breaker integration
type CircuitBreaker interface {
	AllowRequest(key string) bool
	RecordSuccess(key string)
	RecordFailure(key string)
}

// NewFailoverExecutor creates a new failover executor
func NewFailoverExecutor(rm *RouteManager, cb CircuitBreaker, logger *zap.Logger) *FailoverExecutor {
	return &FailoverExecutor{
		routeManager:   rm,
		circuitBreaker: cb,
		logger:         logger,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ExecuteWithFailover executes a request with failover logic
func (e *FailoverExecutor) ExecuteWithFailover(ctx context.Context, modelID string, executor func(apiKey string) error) error {
	// Level 1: Try primary API key
	err := e.tryPrimaryKey(ctx, modelID, executor)
	if err == nil {
		return nil
	}

	e.logger.Warn("primary key failed, trying backup keys",
		zap.String("model", modelID),
		zap.Error(err))

	// Level 2: Try backup keys for same model
	err = e.tryBackupKeys(ctx, modelID, executor)
	if err == nil {
		return nil
	}

	e.logger.Warn("all keys failed, trying alternative models",
		zap.String("model", modelID),
		zap.Error(err))

	// Level 3: Try alternative models
	err = e.tryAlternativeModels(ctx, modelID, executor)
	if err == nil {
		return nil
	}

	e.logger.Error("all alternatives failed, using fallback model",
		zap.String("model", modelID),
		zap.Error(err))

	// Level 4: Try fallback model
	return e.tryFallbackModel(ctx, modelID, executor)
}

// tryPrimaryKey tries the primary API key
func (e *FailoverExecutor) tryPrimaryKey(ctx context.Context, modelID string, executor func(apiKey string) error) error {
	apiKey, err := e.routeManager.SelectAPIKey(modelID)
	if err != nil {
		return err
	}

	// Check circuit breaker
	if !e.circuitBreaker.AllowRequest(apiKey.Key) {
		return fmt.Errorf("circuit breaker open for key: %s", maskKey(apiKey.Key))
	}

	// Execute request
	err = executor(apiKey.Key)

	// Record result
	if err != nil {
		e.circuitBreaker.RecordFailure(apiKey.Key)
		return err
	}

	e.circuitBreaker.RecordSuccess(apiKey.Key)
	return nil
}

// tryBackupKeys tries backup keys for the same model
func (e *FailoverExecutor) tryBackupKeys(ctx context.Context, modelID string, executor func(apiKey string) error) error {
	rule, err := e.routeManager.GetRule(modelID)
	if err != nil {
		return err
	}

	// Try all keys except the first one
	for i := 1; i < len(rule.APIKeys); i++ {
		key := &rule.APIKeys[i]
		if key.Status != KeyStatusActive {
			continue
		}

		if !e.circuitBreaker.AllowRequest(key.Key) {
			continue
		}

		err := executor(key.Key)
		if err == nil {
			e.circuitBreaker.RecordSuccess(key.Key)
			return nil
		}

		e.circuitBreaker.RecordFailure(key.Key)
	}

	return fmt.Errorf("all backup keys failed for model: %s", modelID)
}

// tryAlternativeModels tries alternative models from fallback chain
func (e *FailoverExecutor) tryAlternativeModels(ctx context.Context, modelID string, executor func(apiKey string) error) error {
	rule, err := e.routeManager.GetRule(modelID)
	if err != nil {
		return err
	}

	// Try fallback models
	for _, fallback := range rule.FallbackChain {
		if fallback.Level < 2 {
			continue // Skip non-alternative levels
		}

		// Try to get an API key for alternative model
		apiKey, err := e.routeManager.SelectAPIKey(fallback.ModelID)
		if err != nil {
			continue
		}

		if !e.circuitBreaker.AllowRequest(apiKey.Key) {
			continue
		}

		err = executor(apiKey.Key)
		if err == nil {
			e.circuitBreaker.RecordSuccess(apiKey.Key)
			e.logger.Info("request succeeded with alternative model",
				zap.String("original_model", modelID),
				zap.String("alternative_model", fallback.ModelID))
			return nil
		}

		e.circuitBreaker.RecordFailure(apiKey.Key)
	}

	return fmt.Errorf("all alternative models failed for model: %s", modelID)
}

// tryFallbackModel tries the fallback model
func (e *FailoverExecutor) tryFallbackModel(ctx context.Context, modelID string, executor func(apiKey string) error) error {
	rule, err := e.routeManager.GetRule(modelID)
	if err != nil {
		return err
	}

	// Find fallback model (level 3)
	for _, fallback := range rule.FallbackChain {
		if fallback.Level == 3 {
			apiKey, err := e.routeManager.SelectAPIKey(fallback.ModelID)
			if err != nil {
				continue
			}

			if !e.circuitBreaker.AllowRequest(apiKey.Key) {
				continue
			}

			err = executor(apiKey.Key)
			if err == nil {
				e.circuitBreaker.RecordSuccess(apiKey.Key)
				e.logger.Info("request succeeded with fallback model",
					zap.String("original_model", modelID),
					zap.String("fallback_model", fallback.ModelID))
				return nil
			}

			e.circuitBreaker.RecordFailure(apiKey.Key)
		}
	}

	return fmt.Errorf("no fallback model available for model: %s", modelID)
}

// maskKey masks sensitive API key for logging
func maskKey(key string) string {
	if len(key) < 8 {
		return "***"
	}
	return key[:4] + "***" + key[len(key)-4:]
}
