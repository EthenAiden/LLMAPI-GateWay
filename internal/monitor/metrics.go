package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// Request metrics
	RequestTotal   prometheus.Counter
	RequestErrors  prometheus.Counter
	RequestLatency prometheus.Histogram
	RequestSize    prometheus.Histogram
	ResponseSize   prometheus.Histogram

	// Token metrics
	TokensUsed     prometheus.Counter
	TokensReserved prometheus.Gauge
	QuotaExceeded  prometheus.Counter

	// Circuit breaker metrics
	CircuitBreakerOpen     prometheus.Counter
	CircuitBreakerClosed   prometheus.Counter
	CircuitBreakerHalfOpen prometheus.Counter

	// Rate limiter metrics
	RateLimitExceeded prometheus.Counter

	// Failover metrics
	FailoverAttempts prometheus.Counter
	FailoverSuccess  prometheus.Counter
	FailoverFailure  prometheus.Counter

	// Provider metrics
	ProviderRequests prometheus.CounterVec
	ProviderErrors   prometheus.CounterVec
	ProviderLatency  prometheus.HistogramVec

	// System metrics
	ActiveConnections prometheus.Gauge
	Goroutines        prometheus.Gauge
}

// NewMetrics creates new Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		RequestTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of requests",
		}),
		RequestErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_request_errors_total",
			Help: "Total number of request errors",
		}),
		RequestLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_request_latency_seconds",
			Help:    "Request latency in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		RequestSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_request_size_bytes",
			Help:    "Request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		}),
		ResponseSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_response_size_bytes",
			Help:    "Response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		}),
		TokensUsed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_tokens_used_total",
			Help: "Total tokens used",
		}),
		TokensReserved: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_tokens_reserved",
			Help: "Currently reserved tokens",
		}),
		QuotaExceeded: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_quota_exceeded_total",
			Help: "Total quota exceeded errors",
		}),
		CircuitBreakerOpen: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_circuit_breaker_open_total",
			Help: "Total circuit breaker open events",
		}),
		CircuitBreakerClosed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_circuit_breaker_closed_total",
			Help: "Total circuit breaker closed events",
		}),
		CircuitBreakerHalfOpen: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_circuit_breaker_half_open_total",
			Help: "Total circuit breaker half-open events",
		}),
		RateLimitExceeded: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_rate_limit_exceeded_total",
			Help: "Total rate limit exceeded errors",
		}),
		FailoverAttempts: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_failover_attempts_total",
			Help: "Total failover attempts",
		}),
		FailoverSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_failover_success_total",
			Help: "Total successful failovers",
		}),
		FailoverFailure: promauto.NewCounter(prometheus.CounterOpts{
			Name: "gateway_failover_failure_total",
			Help: "Total failed failovers",
		}),
		ProviderRequests: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_provider_requests_total",
			Help: "Total requests per provider",
		}, []string{"provider"}),
		ProviderErrors: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_provider_errors_total",
			Help: "Total errors per provider",
		}, []string{"provider"}),
		ProviderLatency: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_provider_latency_seconds",
			Help:    "Provider latency in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"provider"}),
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_active_connections",
			Help: "Number of active connections",
		}),
		Goroutines: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_goroutines",
			Help: "Number of goroutines",
		}),
	}
}
