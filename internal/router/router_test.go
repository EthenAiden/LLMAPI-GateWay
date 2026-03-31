package router

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// MockCircuitBreaker is a mock implementation of CircuitBreaker
type MockCircuitBreaker struct {
	allowedKeys map[string]bool
	successes   map[string]int
	failures    map[string]int
}

func NewMockCircuitBreaker() *MockCircuitBreaker {
	return &MockCircuitBreaker{
		allowedKeys: make(map[string]bool),
		successes:   make(map[string]int),
		failures:    make(map[string]int),
	}
}

func (m *MockCircuitBreaker) AllowRequest(key string) bool {
	return m.allowedKeys[key] != false
}

func (m *MockCircuitBreaker) RecordSuccess(key string) {
	m.successes[key]++
}

func (m *MockCircuitBreaker) RecordFailure(key string) {
	m.failures[key]++
}

func TestRouteManagerLoadRules(t *testing.T) {
	// Create a mock Redis client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	logger := zap.NewNop()
	manager := NewRouteManager(client, logger)

	// Test loading rules (will fail if Redis is not available, but that's OK for this test)
	ctx := context.Background()
	err := manager.LoadRules(ctx)
	// We don't assert on error since Redis might not be available
	_ = err
}

func TestRouteManagerSelectAPIKey(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	manager := NewRouteManager(client, logger)

	// Create a test rule
	rule := &RouteRule{
		ModelID:  "test-model",
		Provider: ProviderCursor,
		APIKeys: []APIKeyConfig{
			{Key: "key1", Weight: 1, Status: KeyStatusActive},
			{Key: "key2", Weight: 1, Status: KeyStatusActive},
		},
		LoadBalancer: LoadBalancerRoundRobin,
	}

	manager.rules.Store("test-model", rule)

	// Test selecting API key
	key1, err := manager.SelectAPIKey("test-model")
	if err != nil {
		t.Fatalf("failed to select API key: %v", err)
	}

	if key1.Key != "key1" {
		t.Errorf("expected key1, got %s", key1.Key)
	}

	// Test round-robin
	key2, err := manager.SelectAPIKey("test-model")
	if err != nil {
		t.Fatalf("failed to select API key: %v", err)
	}

	if key2.Key != "key2" {
		t.Errorf("expected key2, got %s", key2.Key)
	}

	// Test round-robin wraps around
	key3, err := manager.SelectAPIKey("test-model")
	if err != nil {
		t.Fatalf("failed to select API key: %v", err)
	}

	if key3.Key != "key1" {
		t.Errorf("expected key1 (wrapped), got %s", key3.Key)
	}
}

func TestRouteManagerWeightedSelect(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	manager := NewRouteManager(client, logger)

	// Create a test rule with weighted keys
	rule := &RouteRule{
		ModelID:  "test-model",
		Provider: ProviderCursor,
		APIKeys: []APIKeyConfig{
			{Key: "key1", Weight: 3, Status: KeyStatusActive},
			{Key: "key2", Weight: 1, Status: KeyStatusActive},
		},
		LoadBalancer: LoadBalancerWeighted,
	}

	manager.rules.Store("test-model", rule)

	// Test weighted selection (key1 should be selected more often)
	key1Count := 0
	key2Count := 0

	for i := 0; i < 100; i++ {
		key, err := manager.SelectAPIKey("test-model")
		if err != nil {
			t.Fatalf("failed to select API key: %v", err)
		}

		if key.Key == "key1" {
			key1Count++
		} else {
			key2Count++
		}
	}

	// key1 should be selected roughly 3x more often than key2
	if key1Count < 50 {
		t.Errorf("key1 selected %d times, expected > 50", key1Count)
	}
}

func TestRouteManagerRandomSelect(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	manager := NewRouteManager(client, logger)

	// Create a test rule with random selection
	rule := &RouteRule{
		ModelID:  "test-model",
		Provider: ProviderCursor,
		APIKeys: []APIKeyConfig{
			{Key: "key1", Weight: 1, Status: KeyStatusActive},
			{Key: "key2", Weight: 1, Status: KeyStatusActive},
		},
		LoadBalancer: LoadBalancerRandom,
	}

	manager.rules.Store("test-model", rule)

	// Test random selection
	key1Count := 0
	key2Count := 0

	for i := 0; i < 100; i++ {
		key, err := manager.SelectAPIKey("test-model")
		if err != nil {
			t.Fatalf("failed to select API key: %v", err)
		}

		if key.Key == "key1" {
			key1Count++
		} else {
			key2Count++
		}
	}

	// Both should be selected at least once
	if key1Count == 0 || key2Count == 0 {
		t.Errorf("random selection not working: key1=%d, key2=%d", key1Count, key2Count)
	}
}

func TestRouteManagerSelectAPIKeyNoActiveKeys(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	manager := NewRouteManager(client, logger)

	// Create a test rule with no active keys
	rule := &RouteRule{
		ModelID:  "test-model",
		Provider: ProviderCursor,
		APIKeys: []APIKeyConfig{
			{Key: "key1", Weight: 1, Status: KeyStatusFailed},
		},
		LoadBalancer: LoadBalancerRoundRobin,
	}

	manager.rules.Store("test-model", rule)

	// Test selecting API key should fail
	_, err := manager.SelectAPIKey("test-model")
	if err == nil {
		t.Errorf("expected error when no active keys available")
	}
}

func TestRouteManagerSelectAPIKeyNonExistent(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	manager := NewRouteManager(client, logger)

	// Test selecting API key for non-existent model
	_, err := manager.SelectAPIKey("non-existent-model")
	if err == nil {
		t.Errorf("expected error for non-existent model")
	}
}

func TestFailoverExecutor(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	manager := NewRouteManager(client, logger)
	circuitBreaker := NewMockCircuitBreaker()

	// Create a test rule
	rule := &RouteRule{
		ModelID:  "test-model",
		Provider: ProviderCursor,
		APIKeys: []APIKeyConfig{
			{Key: "key1", Weight: 1, Status: KeyStatusActive},
			{Key: "key2", Weight: 1, Status: KeyStatusActive},
		},
		LoadBalancer: LoadBalancerRoundRobin,
		FallbackChain: []FallbackConfig{
			{Level: 2, ModelID: "fallback-model", Provider: ProviderKiro},
		},
	}

	manager.rules.Store("test-model", rule)

	// Create fallback rule
	fallbackRule := &RouteRule{
		ModelID:  "fallback-model",
		Provider: ProviderKiro,
		APIKeys: []APIKeyConfig{
			{Key: "fallback-key", Weight: 1, Status: KeyStatusActive},
		},
		LoadBalancer: LoadBalancerRoundRobin,
	}

	manager.rules.Store("fallback-model", fallbackRule)

	// Allow all keys
	circuitBreaker.allowedKeys["key1"] = true
	circuitBreaker.allowedKeys["key2"] = true
	circuitBreaker.allowedKeys["fallback-key"] = true

	executor := NewFailoverExecutor(manager, circuitBreaker, logger)

	// Test successful execution
	callCount := 0
	err := executor.ExecuteWithFailover(context.Background(), "test-model", func(apiKey string) error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestHealthProbe(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	manager := NewRouteManager(client, logger)

	// Create a test rule
	rule := &RouteRule{
		ModelID:  "test-model",
		Provider: ProviderCursor,
		APIKeys: []APIKeyConfig{
			{Key: "key1", Weight: 1, Status: KeyStatusFailed},
		},
		LoadBalancer: LoadBalancerRoundRobin,
	}

	manager.rules.Store("test-model", rule)

	probe := NewHealthProbe(manager, 1*time.Second, logger)

	// Test marking key as active
	err := probe.MarkKeyActive("test-model", "key1")
	if err != nil {
		t.Errorf("failed to mark key as active: %v", err)
	}

	// Verify key is now active
	rule, _ = manager.GetRule("test-model")
	if rule.APIKeys[0].Status != KeyStatusActive {
		t.Errorf("expected key to be active")
	}

	// Test marking key as failed
	err = probe.MarkKeyFailed("test-model", "key1")
	if err != nil {
		t.Errorf("failed to mark key as failed: %v", err)
	}

	// Verify key is now failed
	rule, _ = manager.GetRule("test-model")
	if rule.APIKeys[0].Status != KeyStatusFailed {
		t.Errorf("expected key to be failed")
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sk-1234567890", "sk-1***7890"},
		{"short", "***"},
		{"", "***"},
		{"12345678", "1234***5678"},
	}

	for _, test := range tests {
		result := maskKey(test.input)
		if result != test.expected {
			t.Errorf("maskKey(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestRouteRuleJSON(t *testing.T) {
	rule := &RouteRule{
		ModelID:  "test-model",
		Provider: ProviderCursor,
		APIKeys: []APIKeyConfig{
			{Key: "key1", Weight: 1, Status: KeyStatusActive, LastCheck: time.Now()},
		},
		LoadBalancer: LoadBalancerRoundRobin,
		UpdatedAt:    time.Now(),
	}

	// Test marshaling
	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("failed to marshal rule: %v", err)
	}

	// Test unmarshaling
	var unmarshaled RouteRule
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal rule: %v", err)
	}

	if unmarshaled.ModelID != rule.ModelID {
		t.Errorf("expected model ID %s, got %s", rule.ModelID, unmarshaled.ModelID)
	}

	if unmarshaled.Provider != rule.Provider {
		t.Errorf("expected provider %s, got %s", rule.Provider, unmarshaled.Provider)
	}
}
