package router

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// HealthProbe performs health checks on API keys
type HealthProbe struct {
	routeManager *RouteManager
	httpClient   *http.Client
	interval     time.Duration
	logger       *zap.Logger
}

// NewHealthProbe creates a new health probe
func NewHealthProbe(rm *RouteManager, interval time.Duration, logger *zap.Logger) *HealthProbe {
	return &HealthProbe{
		routeManager: rm,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		interval: interval,
		logger:   logger,
	}
}

// Start starts the health probe
func (p *HealthProbe) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("health probe stopped")
			return
		case <-ticker.C:
			p.probeAll(ctx)
		}
	}
}

// probeAll probes all API keys
func (p *HealthProbe) probeAll(ctx context.Context) {
	p.routeManager.rules.Range(func(key, value interface{}) bool {
		rule := value.(*RouteRule)

		for i := range rule.APIKeys {
			apiKey := &rule.APIKeys[i]

			// Try to recover failed keys
			if apiKey.Status == KeyStatusFailed {
				if p.probeKey(ctx, apiKey, rule.Provider) {
					apiKey.Status = KeyStatusActive
					apiKey.LastCheck = time.Now()
					p.logger.Info("API key recovered",
						zap.String("key", maskKey(apiKey.Key)),
						zap.String("model", rule.ModelID))
				}
			}
		}

		return true
	})
}

// probeKey probes a single API key
func (p *HealthProbe) probeKey(ctx context.Context, apiKey *APIKeyConfig, provider ProviderType) bool {
	// Simple health check - just verify the key format is valid
	// In production, this would make actual API calls
	if apiKey.Key == "" {
		return false
	}

	apiKey.LastCheck = time.Now()
	return true
}

// MarkKeyFailed marks an API key as failed
func (p *HealthProbe) MarkKeyFailed(modelID string, apiKey string) error {
	rule, err := p.routeManager.GetRule(modelID)
	if err != nil {
		return err
	}

	for i := range rule.APIKeys {
		if rule.APIKeys[i].Key == apiKey {
			rule.APIKeys[i].Status = KeyStatusFailed
			rule.APIKeys[i].LastCheck = time.Now()
			p.logger.Warn("API key marked as failed",
				zap.String("key", maskKey(apiKey)),
				zap.String("model", modelID))
			break
		}
	}

	return nil
}

// MarkKeyActive marks an API key as active
func (p *HealthProbe) MarkKeyActive(modelID string, apiKey string) error {
	rule, err := p.routeManager.GetRule(modelID)
	if err != nil {
		return err
	}

	for i := range rule.APIKeys {
		if rule.APIKeys[i].Key == apiKey {
			rule.APIKeys[i].Status = KeyStatusActive
			rule.APIKeys[i].LastCheck = time.Now()
			p.logger.Info("API key marked as active",
				zap.String("key", maskKey(apiKey)),
				zap.String("model", modelID))
			break
		}
	}

	return nil
}
