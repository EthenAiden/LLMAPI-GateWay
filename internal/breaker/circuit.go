package breaker

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitBreaker implements circuit breaker pattern with sliding window
type CircuitBreaker struct {
	windows sync.Map // key -> *SlidingWindow
	config  CircuitBreakerConfig
	logger  *zap.Logger
}

// SlidingWindow tracks requests in a sliding time window
type SlidingWindow struct {
	mu              sync.RWMutex
	buckets         []Bucket
	bucketSize      time.Duration
	totalBuckets    int
	state           CircuitState
	lastStateChange time.Time
	halfOpenCount   int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		logger: logger,
	}
}

// newWindow creates a new sliding window
func (cb *CircuitBreaker) newWindow() *SlidingWindow {
	totalBuckets := int(cb.config.WindowSize.Seconds())
	if totalBuckets < 1 {
		totalBuckets = 1
	}

	return &SlidingWindow{
		buckets:         make([]Bucket, totalBuckets),
		bucketSize:      time.Second,
		totalBuckets:    totalBuckets,
		state:           StateClosed,
		lastStateChange: time.Now(),
		halfOpenCount:   0,
	}
}

// AllowRequest checks if a request is allowed
func (cb *CircuitBreaker) AllowRequest(key string) bool {
	windowVal, _ := cb.windows.LoadOrStore(key, cb.newWindow())
	window := windowVal.(*SlidingWindow)

	window.mu.Lock()
	defer window.mu.Unlock()

	switch window.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we can enter half-open state
		if time.Since(window.lastStateChange) > cb.config.HalfOpenTimeout {
			window.state = StateHalfOpen
			window.lastStateChange = time.Now()
			window.halfOpenCount = 0
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		if window.halfOpenCount < cb.config.MaxRequests {
			window.halfOpenCount++
			return true
		}
		return false
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess(key string) {
	windowVal, _ := cb.windows.Load(key)
	if windowVal == nil {
		return
	}

	window := windowVal.(*SlidingWindow)

	window.mu.Lock()
	defer window.mu.Unlock()

	// Record success
	window.record(true)

	// If in half-open state, check if we can close the breaker
	if window.state == StateHalfOpen {
		errorRate := window.calculateErrorRate()
		if errorRate < cb.config.ErrorThreshold {
			window.state = StateClosed
			window.lastStateChange = time.Now()
			cb.logger.Info("circuit breaker closed", zap.String("key", key))
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure(key string) {
	windowVal, _ := cb.windows.LoadOrStore(key, cb.newWindow())
	window := windowVal.(*SlidingWindow)

	window.mu.Lock()
	defer window.mu.Unlock()

	// Record failure
	window.record(false)

	// Calculate error rate
	errorRate := window.calculateErrorRate()

	// If error rate exceeds threshold and breaker is closed, open it
	if errorRate > cb.config.ErrorThreshold && window.state == StateClosed {
		window.state = StateOpen
		window.lastStateChange = time.Now()
		cb.logger.Warn("circuit breaker opened",
			zap.String("key", key),
			zap.Float64("error_rate", errorRate))
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState(key string) CircuitState {
	windowVal, ok := cb.windows.Load(key)
	if !ok {
		return StateClosed
	}

	window := windowVal.(*SlidingWindow)
	window.mu.RLock()
	defer window.mu.RUnlock()

	return window.state
}

// record records a request result in the sliding window
func (w *SlidingWindow) record(success bool) {
	now := time.Now()
	bucketIndex := int(now.Unix()/int64(w.bucketSize.Seconds())) % w.totalBuckets

	bucket := &w.buckets[bucketIndex]

	// If this is a new time bucket, reset counters
	if now.Sub(bucket.Timestamp) > w.bucketSize {
		bucket.Timestamp = now
		bucket.Success = 0
		bucket.Failure = 0
	}

	if success {
		bucket.Success++
	} else {
		bucket.Failure++
	}
}

// calculateErrorRate calculates the error rate in the sliding window
func (w *SlidingWindow) calculateErrorRate() float64 {
	now := time.Now()
	var totalSuccess, totalFailure int64

	for i := range w.buckets {
		bucket := &w.buckets[i]
		// Only count data within the window
		if now.Sub(bucket.Timestamp) <= time.Duration(w.totalBuckets)*w.bucketSize {
			totalSuccess += bucket.Success
			totalFailure += bucket.Failure
		}
	}

	total := totalSuccess + totalFailure
	if total == 0 {
		return 0
	}

	return float64(totalFailure) / float64(total)
}
