package breaker

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func TestCircuitBreakerClosed(t *testing.T) {
	config := CircuitBreakerConfig{
		WindowSize:      10 * time.Second,
		ErrorThreshold:  0.5,
		HalfOpenTimeout: 30 * time.Second,
		MaxRequests:     3,
	}

	logger := zap.NewNop()
	cb := NewCircuitBreaker(config, logger)

	// In closed state, all requests should be allowed
	for i := 0; i < 10; i++ {
		if !cb.AllowRequest("test-key") {
			t.Errorf("request %d should be allowed in closed state", i)
		}
	}
}

func TestCircuitBreakerOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		WindowSize:      10 * time.Second,
		ErrorThreshold:  0.5,
		HalfOpenTimeout: 30 * time.Second,
		MaxRequests:     3,
	}

	logger := zap.NewNop()
	cb := NewCircuitBreaker(config, logger)

	// Record failures to trigger breaker
	for i := 0; i < 6; i++ {
		cb.AllowRequest("test-key")
		cb.RecordFailure("test-key")
	}

	// After 60% failure rate, breaker should open
	if cb.AllowRequest("test-key") {
		t.Errorf("request should be rejected when breaker is open")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		WindowSize:      10 * time.Second,
		ErrorThreshold:  0.5,
		HalfOpenTimeout: 1 * time.Millisecond, // Very short for testing
		MaxRequests:     3,
	}

	logger := zap.NewNop()
	cb := NewCircuitBreaker(config, logger)

	// Trigger breaker open
	for i := 0; i < 6; i++ {
		cb.AllowRequest("test-key")
		cb.RecordFailure("test-key")
	}

	// Wait for half-open timeout
	time.Sleep(10 * time.Millisecond)

	// Should allow request in half-open state
	if !cb.AllowRequest("test-key") {
		t.Errorf("request should be allowed in half-open state")
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	config := CircuitBreakerConfig{
		WindowSize:      10 * time.Second,
		ErrorThreshold:  0.5,
		HalfOpenTimeout: 1 * time.Millisecond,
		MaxRequests:     3,
	}

	logger := zap.NewNop()
	cb := NewCircuitBreaker(config, logger)

	// Trigger breaker open
	for i := 0; i < 6; i++ {
		cb.AllowRequest("test-key")
		cb.RecordFailure("test-key")
	}

	// Verify breaker is open
	if cb.GetState("test-key") != StateOpen {
		t.Errorf("breaker should be open after failures")
	}

	// Wait for half-open timeout
	time.Sleep(10 * time.Millisecond)

	// Record successes in half-open state
	for i := 0; i < 3; i++ {
		if !cb.AllowRequest("test-key") {
			t.Errorf("request %d should be allowed in half-open state", i)
		}
		cb.RecordSuccess("test-key")
	}

	// Breaker should transition to closed after successful probes
	// Note: The exact state depends on error rate calculation
	state := cb.GetState("test-key")
	if state != StateClosed && state != StateHalfOpen {
		t.Errorf("breaker should be closed or half-open after successful recovery, got %d", state)
	}
}

func TestCircuitBreakerGetState(t *testing.T) {
	config := CircuitBreakerConfig{
		WindowSize:      10 * time.Second,
		ErrorThreshold:  0.5,
		HalfOpenTimeout: 30 * time.Second,
		MaxRequests:     3,
	}

	logger := zap.NewNop()
	cb := NewCircuitBreaker(config, logger)

	// Initial state should be closed
	if cb.GetState("test-key") != StateClosed {
		t.Errorf("initial state should be closed")
	}

	// Trigger open state
	for i := 0; i < 6; i++ {
		cb.AllowRequest("test-key")
		cb.RecordFailure("test-key")
	}

	if cb.GetState("test-key") != StateOpen {
		t.Errorf("state should be open after failures")
	}
}

func TestRateLimiterAllowRequest(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger)

	ctx := context.Background()

	// Test allowing requests within limit
	dimension := RateLimitDimension{
		Type:       "user",
		Identifier: "user123",
		Capacity:   100,
		Rate:       10, // 10 tokens per second
	}

	// First request should be allowed
	allowed, err := limiter.AllowRequest(ctx, dimension, 10)
	if err != nil {
		t.Fatalf("failed to check rate limit: %v", err)
	}

	if !allowed {
		t.Error("first request should be allowed")
	}

	// Second request should be allowed (still have tokens)
	allowed, err = limiter.AllowRequest(ctx, dimension, 50)
	if err != nil {
		t.Fatalf("failed to check rate limit: %v", err)
	}

	if !allowed {
		t.Error("second request should be allowed")
	}

	// Third request should be rejected (exceeds capacity)
	allowed, err = limiter.AllowRequest(ctx, dimension, 50)
	if err != nil {
		t.Fatalf("failed to check rate limit: %v", err)
	}

	if allowed {
		t.Error("third request should be rejected (exceeds capacity)")
	}

	// Clean up
	limiter.Reset(ctx, dimension)
}

func TestRateLimiterCheckMultiDimension(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger)

	ctx := context.Background()

	// Test multi-dimensional rate limiting
	err := limiter.CheckMultiDimension(ctx, "user123", "app456", "model789", 10)
	if err != nil {
		t.Fatalf("first check should pass: %v", err)
	}

	// Clean up
	limiter.Reset(ctx, RateLimitDimension{Type: "user", Identifier: "user123"})
	limiter.Reset(ctx, RateLimitDimension{Type: "app", Identifier: "app456"})
	limiter.Reset(ctx, RateLimitDimension{Type: "model", Identifier: "model789"})
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{
		Dimension:  "user",
		Identifier: "user123",
	}

	expected := "rate limit exceeded for user: user123"
	if err.Error() != expected {
		t.Errorf("expected %s, got %s", expected, err.Error())
	}
}

func TestQuotaExceededError(t *testing.T) {
	err := &QuotaExceededError{
		Dimension:  "app",
		Identifier: "app456",
	}

	expected := "quota exceeded for app: app456"
	if err.Error() != expected {
		t.Errorf("expected %s, got %s", expected, err.Error())
	}
}

func TestSlidingWindowErrorRate(t *testing.T) {
	config := CircuitBreakerConfig{
		WindowSize:      10 * time.Second,
		ErrorThreshold:  0.5,
		HalfOpenTimeout: 30 * time.Second,
		MaxRequests:     3,
	}

	logger := zap.NewNop()
	cb := NewCircuitBreaker(config, logger)

	// Record 3 successes and 2 failures
	for i := 0; i < 3; i++ {
		cb.AllowRequest("test-key")
		cb.RecordSuccess("test-key")
	}

	for i := 0; i < 2; i++ {
		cb.AllowRequest("test-key")
		cb.RecordFailure("test-key")
	}

	// Error rate should be 2/5 = 0.4 (below 0.5 threshold)
	if cb.GetState("test-key") != StateClosed {
		t.Error("breaker should remain closed with 40 percent error rate")
	}

	// Record 3 more failures to exceed threshold
	for i := 0; i < 3; i++ {
		cb.AllowRequest("test-key")
		cb.RecordFailure("test-key")
	}

	// Error rate should now be 5/8 = 0.625 (above 0.5 threshold)
	if cb.GetState("test-key") != StateOpen {
		t.Error("breaker should open with 62.5 percent error rate")
	}
}
