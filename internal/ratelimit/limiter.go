package ratelimit

import (
	"context"

	"go.uber.org/zap"
)

// RateLimiter 限流器
type RateLimiter struct {
	logger *zap.Logger
}

// NewRateLimiter 创建限流器
func NewRateLimiter(logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		logger: logger,
	}
}

// AllowRequest 检查是否允许请求
func (rl *RateLimiter) AllowRequest(ctx context.Context, dimension RateLimitDimension, tokens int) (bool, error) {
	// TODO: 实现 Redis Lua 令牌桶限流逻辑
	return true, nil
}

// RateLimitDimension 限流维度
type RateLimitDimension struct {
	Type       string  // user / app / model
	Identifier string  // 具体标识
	Capacity   int     // 桶容量
	Rate       float64 // 令牌生成速率（个/秒）
}
