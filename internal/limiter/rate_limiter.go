package limiter

import (
	"context"
	"fmt"
	"partivo_tickets/internal/provider"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter is a thread-safe, distributed rate limiter using Redis and Lua script.
type RedisRateLimiter struct {
	redisClient    *redis.Client
	namespace      provider.RedisNamespace // Namespace for Redis keys.
	rate           float64                 // Tokens added per second.
	bucketSize     float64                 // How many tokens the bucket can hold.
	keyExpiration  time.Duration           // Expiration time for the Redis key.
	luaScript      *redis.Script           // Compiled Lua script for atomicity.
}

// NewRedisRateLimiter creates a new Redis-based rate limiter.
func NewRedisRateLimiter(redisClient *redis.Client, ns provider.RedisNamespace, rate float64, size float64, expiration time.Duration) *RedisRateLimiter {
	// This Lua script implements the token bucket algorithm atomically.
	const script = `
		local tokens_key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local requested = tonumber(ARGV[4])
		local expire_seconds = math.ceil(tonumber(ARGV[5]))

		local bucket = redis.call("HGETALL", tokens_key)
		local tokens
		local last_refill

		if #bucket == 0 then
			tokens = capacity
			last_refill = now
		else
			for i = 1, #bucket, 2 do
				if bucket[i] == "tokens" then
					tokens = tonumber(bucket[i+1])
				elseif bucket[i] == "last_refill" then
					last_refill = tonumber(bucket[i+1])
				end
			end
			
			local elapsed = now - last_refill
			tokens = tokens + elapsed * rate
			last_refill = now
			if tokens > capacity then
				tokens = capacity
			end
		end

		if tokens >= requested then
			tokens = tokens - requested
			redis.call("HSET", tokens_key, "tokens", tokens, "last_refill", last_refill)
			redis.call("EXPIRE", tokens_key, expire_seconds)
			return 1 -- Allowed
		else
			return 0 -- Denied
		end
	`

	return &RedisRateLimiter{
		redisClient:    redisClient,
		namespace:      ns,
		rate:           rate,
		bucketSize:     size,
		keyExpiration:  expiration,
		luaScript:      redis.NewScript(script),
	}
}

// Allow checks if a request from the given identifier is allowed.
func (l *RedisRateLimiter) Allow(ctx context.Context, identifier string) (bool, error) {
	key := fmt.Sprintf("%sratelimit:%s", l.namespace, identifier)

	now := float64(time.Now().UnixNano()) / 1e9

	result, err := l.luaScript.Run(ctx, l.redisClient, []string{key}, l.rate, l.bucketSize, now, 1.0, l.keyExpiration.Seconds()).Result()
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit script: %w", err)
	}

	return result == int64(1), nil
}
