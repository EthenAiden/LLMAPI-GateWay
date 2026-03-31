package breaker

import "time"

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Breaker is open, rejecting requests
	StateHalfOpen                     // Testing if service recovered
)

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	WindowSize      time.Duration // Time window for error rate calculation
	ErrorThreshold  float64       // Error rate threshold (0.0-1.0)
	HalfOpenTimeout time.Duration // Time to wait before entering half-open state
	MaxRequests     int           // Max requests in half-open state
}

// Bucket represents a time bucket for statistics
type Bucket struct {
	Timestamp time.Time
	Success   int64
	Failure   int64
}

// RateLimitDimension represents a rate limiting dimension
type RateLimitDimension struct {
	Type       string  // user / app / model
	Identifier string  // Specific identifier
	Capacity   int     // Bucket capacity
	Rate       float64 // Token generation rate (tokens/second)
}

// RateLimitError represents a rate limit error
type RateLimitError struct {
	Dimension  string
	Identifier string
}

func (e *RateLimitError) Error() string {
	return "rate limit exceeded for " + e.Dimension + ": " + e.Identifier
}

// QuotaExceededError represents a quota exceeded error
type QuotaExceededError struct {
	Dimension  string
	Identifier string
}

func (e *QuotaExceededError) Error() string {
	return "quota exceeded for " + e.Dimension + ": " + e.Identifier
}
