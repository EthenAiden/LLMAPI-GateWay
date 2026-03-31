package router

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RouteManager manages routing rules and updates
type RouteManager struct {
	redis      *redis.Client
	rules      sync.Map // modelID -> *RouteRule
	subscriber *redis.PubSub
	logger     *zap.Logger
	mu         sync.RWMutex
	roundRobin map[string]int // modelID -> current index for round robin
}

// NewRouteManager creates a new route manager
func NewRouteManager(redisClient *redis.Client, logger *zap.Logger) *RouteManager {
	return &RouteManager{
		redis:      redisClient,
		logger:     logger,
		roundRobin: make(map[string]int),
	}
}

// LoadRules loads all routing rules from Redis
func (m *RouteManager) LoadRules(ctx context.Context) error {
	keys, err := m.redis.Keys(ctx, "route:*").Result()
	if err != nil {
		m.logger.Error("failed to get route keys", zap.Error(err))
		return err
	}

	for _, key := range keys {
		data, err := m.redis.Get(ctx, key).Result()
		if err != nil {
			m.logger.Warn("failed to get route rule", zap.String("key", key), zap.Error(err))
			continue
		}

		var rule RouteRule
		if err := json.Unmarshal([]byte(data), &rule); err != nil {
			m.logger.Warn("failed to unmarshal route rule", zap.String("key", key), zap.Error(err))
			continue
		}

		m.rules.Store(rule.ModelID, &rule)
		m.logger.Debug("loaded route rule", zap.String("model", rule.ModelID))
	}

	return nil
}

// SubscribeUpdates subscribes to route update events via Redis Pub/Sub
func (m *RouteManager) SubscribeUpdates(ctx context.Context) error {
	m.subscriber = m.redis.Subscribe(ctx, "route:updates")

	go func() {
		for msg := range m.subscriber.Channel() {
			var update RouteUpdateEvent
			if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
				m.logger.Error("failed to parse route update", zap.Error(err))
				continue
			}

			// Update local cache
			m.rules.Store(update.ModelID, update.Rule)
			m.logger.Info("route rule updated", zap.String("model", update.ModelID))
		}
	}()

	return nil
}

// GetRule retrieves a routing rule for a model
func (m *RouteManager) GetRule(modelID string) (*RouteRule, error) {
	ruleVal, ok := m.rules.Load(modelID)
	if !ok {
		return nil, fmt.Errorf("no route rule for model: %s", modelID)
	}

	rule := ruleVal.(*RouteRule)
	return rule, nil
}

// SelectAPIKey selects an API key based on load balancing strategy
func (m *RouteManager) SelectAPIKey(modelID string) (*APIKeyConfig, error) {
	rule, err := m.GetRule(modelID)
	if err != nil {
		return nil, err
	}

	// Filter active API keys
	activeKeys := make([]*APIKeyConfig, 0)
	for i := range rule.APIKeys {
		if rule.APIKeys[i].Status == KeyStatusActive {
			activeKeys = append(activeKeys, &rule.APIKeys[i])
		}
	}

	if len(activeKeys) == 0 {
		return nil, fmt.Errorf("no active API key for model: %s", modelID)
	}

	// Select based on load balancing strategy
	switch rule.LoadBalancer {
	case LoadBalancerRoundRobin:
		return m.roundRobinSelect(modelID, activeKeys), nil
	case LoadBalancerRandom:
		return activeKeys[rand.Intn(len(activeKeys))], nil
	case LoadBalancerWeighted:
		return m.weightedSelect(activeKeys), nil
	default:
		return activeKeys[0], nil
	}
}

// roundRobinSelect selects an API key using round-robin algorithm
func (m *RouteManager) roundRobinSelect(modelID string, keys []*APIKeyConfig) *APIKeyConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	index := m.roundRobin[modelID]
	selected := keys[index%len(keys)]
	m.roundRobin[modelID] = (index + 1) % len(keys)

	return selected
}

// weightedSelect selects an API key using weighted algorithm
func (m *RouteManager) weightedSelect(keys []*APIKeyConfig) *APIKeyConfig {
	totalWeight := 0
	for _, key := range keys {
		totalWeight += key.Weight
	}

	if totalWeight == 0 {
		return keys[rand.Intn(len(keys))]
	}

	randVal := rand.Intn(totalWeight)
	cumWeight := 0

	for _, key := range keys {
		cumWeight += key.Weight
		if randVal < cumWeight {
			return key
		}
	}

	return keys[len(keys)-1]
}

// UpdateRule updates a routing rule in Redis
func (m *RouteManager) UpdateRule(ctx context.Context, rule *RouteRule) error {
	rule.UpdatedAt = time.Now()

	data, err := json.Marshal(rule)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("route:%s", rule.ModelID)
	if err := m.redis.Set(ctx, key, data, 0).Err(); err != nil {
		return err
	}

	// Publish update event
	event := RouteUpdateEvent{
		ModelID: rule.ModelID,
		Rule:    rule,
	}
	eventData, _ := json.Marshal(event)
	m.redis.Publish(ctx, "route:updates", eventData)

	m.logger.Info("route rule updated", zap.String("model", rule.ModelID))
	return nil
}

// Close closes the route manager
func (m *RouteManager) Close() error {
	if m.subscriber != nil {
		return m.subscriber.Close()
	}
	return nil
}
