package breaker

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RateLimiter implements distributed rate limiting using Redis Lua scripts
type RateLimiter struct {
	redis    *redis.Client
	script   *redis.Script
	logger   *zap.Logger
	userCap  int
	userRate float64
	appCap   int
	appRate  float64
	modelCap int
	modelRate float64
}

// RateLimiterConfig holds per-dimension rate limit configuration
type RateLimiterConfig struct {
	UserCapacity  int
	UserRate      float64
	AppCapacity   int
	AppRate       float64
	ModelCapacity int
	ModelRate     float64
}

// Token bucket algorithm Lua script
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_update')
local tokens = tonumber(bucket[1])
local last_update = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    last_update = now
end

-- Calculate new tokens based on elapsed time
local elapsed = now - last_update
local new_tokens = elapsed * rate
tokens = math.min(capacity, tokens + new_tokens)

-- Try to consume tokens
if tokens >= requested then
    tokens = tokens - requested
    redis.call('HMSET', key, 'tokens', tokens, 'last_update', now)
    redis.call('EXPIRE', key, 3600)
    return 1
else
    return 0
end
`

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, logger *zap.Logger, cfg ...RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		redis:     redisClient,
		script:    redis.NewScript(tokenBucketScript),
		logger:    logger,
		// defaults
		userCap:   10000,
		userRate:  100,
		appCap:    50000,
		appRate:   500,
		modelCap:  100000,
		modelRate: 1000,
	}
	if len(cfg) > 0 {
		c := cfg[0]
		if c.UserCapacity > 0 {
			rl.userCap = c.UserCapacity
		}
		if c.UserRate > 0 {
			rl.userRate = c.UserRate
		}
		if c.AppCapacity > 0 {
			rl.appCap = c.AppCapacity
		}
		if c.AppRate > 0 {
			rl.appRate = c.AppRate
		}
		if c.ModelCapacity > 0 {
			rl.modelCap = c.ModelCapacity
		}
		if c.ModelRate > 0 {
			rl.modelRate = c.ModelRate
		}
	}
	return rl
}

// AllowRequest checks if a request is allowed based on rate limit
func (rl *RateLimiter) AllowRequest(ctx context.Context, dimension RateLimitDimension, tokens int) (bool, error) {
	key := rl.buildKey(dimension)

	result, err := rl.script.Run(ctx, rl.redis, []string{key},
		dimension.Capacity,
		dimension.Rate,
		tokens,
		time.Now().Unix(),
	).Result()

	if err != nil {
		rl.logger.Error("rate limit check failed", zap.Error(err))
		return false, err
	}

	allowed := result.(int64) == 1

	if !allowed {
		rl.logger.Warn("rate limit exceeded",
			zap.String("dimension", dimension.Type),
			zap.String("identifier", dimension.Identifier))
	}

	return allowed, nil
}

// CheckMultiDimension checks rate limits across multiple dimensions
func (rl *RateLimiter) CheckMultiDimension(ctx context.Context, userID, appID, modelID string, tokens int) error {
	dimensions := []RateLimitDimension{
		{Type: "user", Identifier: userID, Capacity: rl.userCap, Rate: rl.userRate},
		{Type: "app", Identifier: appID, Capacity: rl.appCap, Rate: rl.appRate},
		{Type: "model", Identifier: modelID, Capacity: rl.modelCap, Rate: rl.modelRate},
	}

	for _, dim := range dimensions {
		allowed, err := rl.AllowRequest(ctx, dim, tokens)
		if err != nil {
			return err
		}
		if !allowed {
			return &RateLimitError{
				Dimension:  dim.Type,
				Identifier: dim.Identifier,
			}
		}
	}

	return nil
}

// buildKey builds the Redis key for a rate limit dimension
func (rl *RateLimiter) buildKey(dimension RateLimitDimension) string {
	return fmt.Sprintf("ratelimit:%s:%s", dimension.Type, dimension.Identifier)
}

// Reset resets the rate limit for a dimension
func (rl *RateLimiter) Reset(ctx context.Context, dimension RateLimitDimension) error {
	key := rl.buildKey(dimension)
	return rl.redis.Del(ctx, key).Err()
}

// GetTokens gets the current token count for a dimension
func (rl *RateLimiter) GetTokens(ctx context.Context, dimension RateLimitDimension) (float64, error) {
	key := rl.buildKey(dimension)
	result, err := rl.redis.HGet(ctx, key, "tokens").Result()
	if err != nil {
		if err == redis.Nil {
			return float64(dimension.Capacity), nil
		}
		return 0, err
	}

	var tokens float64
	fmt.Sscanf(result, "%f", &tokens)
	return tokens, nil
}
