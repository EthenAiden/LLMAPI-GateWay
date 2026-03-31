package router

import (
	"time"
)

// ProviderType represents upstream service provider type
type ProviderType string

const (
	ProviderCursor      ProviderType = "cursor"
	ProviderKiro        ProviderType = "kiro"
	ProviderAntigravity ProviderType = "antigravity"
)

// KeyStatus represents API key status
type KeyStatus string

const (
	KeyStatusActive   KeyStatus = "active"
	KeyStatusCooldown KeyStatus = "cooldown"
	KeyStatusFailed   KeyStatus = "failed"
)

// LoadBalancerType represents load balancing strategy
type LoadBalancerType string

const (
	LoadBalancerRoundRobin LoadBalancerType = "round_robin"
	LoadBalancerRandom     LoadBalancerType = "random"
	LoadBalancerWeighted   LoadBalancerType = "weighted"
)

// APIKeyConfig represents API key configuration
type APIKeyConfig struct {
	Key       string    `json:"key"`
	Weight    int       `json:"weight"`
	Status    KeyStatus `json:"status"`
	LastCheck time.Time `json:"last_check"`
}

// FallbackConfig represents fallback configuration
type FallbackConfig struct {
	Level     int          `json:"level"`
	ModelID   string       `json:"model_id"`
	Provider  ProviderType `json:"provider"`
	Condition string       `json:"condition"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
}

// RouteRule represents a routing rule for a model
type RouteRule struct {
	ModelID       string            `json:"model_id"`
	Provider      ProviderType      `json:"provider"`
	APIKeys       []APIKeyConfig    `json:"api_keys"`
	FallbackChain []FallbackConfig  `json:"fallback_chain"`
	LoadBalancer  LoadBalancerType  `json:"load_balancer"`
	HealthCheck   HealthCheckConfig `json:"health_check"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// RouteUpdateEvent represents a route update event
type RouteUpdateEvent struct {
	ModelID string     `json:"model_id"`
	Rule    *RouteRule `json:"rule"`
}
