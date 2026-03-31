package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// providerProbeURLs maps provider to its chat completions endpoint
var providerProbeURLs = map[ProviderType]string{
	ProviderCursor:      "https://api.cursor.sh/v1/chat/completions",
	ProviderKiro:        "https://api.kiro.ai/v1/messages",
	ProviderAntigravity: "https://api.antigravity.ai/v1/chat/completions",
}

// minimalProbeBody is the smallest valid request body for probing
var minimalProbeBody = map[string]interface{}{
	"model": "probe",
	"messages": []map[string]string{
		{"role": "user", "content": "ping"},
	},
	"max_tokens": 1,
}

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

// probeKey probes a single API key by making a real HTTP request to the upstream
func (p *HealthProbe) probeKey(ctx context.Context, apiKey *APIKeyConfig, provider ProviderType) bool {
	if apiKey.Key == "" {
		return false
	}

	probeURL, ok := providerProbeURLs[provider]
	if !ok {
		p.logger.Warn("unknown provider for health probe", zap.String("provider", string(provider)))
		return false
	}

	bodyBytes, err := json.Marshal(minimalProbeBody)
	if err != nil {
		return false
	}

	probeCtx, cancel := context.WithTimeout(ctx, p.httpClient.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey.Key))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.logger.Debug("health probe request failed",
			zap.String("provider", string(provider)),
			zap.String("key", maskKey(apiKey.Key)),
			zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	// 401/403 means key is invalid; 429 means key is alive but rate-limited (treat as healthy)
	// 5xx means upstream is down
	alive := resp.StatusCode != http.StatusInternalServerError &&
		resp.StatusCode != http.StatusBadGateway &&
		resp.StatusCode != http.StatusServiceUnavailable &&
		resp.StatusCode != http.StatusGatewayTimeout

	apiKey.LastCheck = time.Now()
	return alive
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
