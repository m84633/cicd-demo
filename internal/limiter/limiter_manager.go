package limiter

import (
	"fmt"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/provider"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultPolicyName = "default"

// Manager holds and provides access to named rate limiters.
type Manager struct {
	limiters map[string]*RedisRateLimiter
}

// NewManager creates a new rate limiter manager from configuration.
// It initializes a rate limiter for the default policy and each named policy.
func NewManager(cfg *conf.RateLimiterConfig, redisClient *redis.Client, ns provider.RedisNamespace) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rate limiter config is nil")
	}

	limiters := make(map[string]*RedisRateLimiter)

	// Helper function to parse a policy and create a limiter
	createLimiter := func(policy conf.RateLimiterPolicy) (*RedisRateLimiter, error) {
		if policy.Limit <= 0 {
			return nil, fmt.Errorf("policy limit must be positive")
		}
		duration, err := time.ParseDuration(policy.Interval)
		if err != nil {
			return nil, fmt.Errorf("invalid policy interval format: %w", err)
		}
		if duration <= 0 {
			return nil, fmt.Errorf("policy interval must be positive")
		}

		// Calculate rate (tokens per second) and size
		rate := float64(policy.Limit) / duration.Seconds()
		size := float64(policy.Limit)
		// Calculate a sensible expiration time, e.g., 2x the interval.
		expiration := duration * 2

		return NewRedisRateLimiter(redisClient, ns, rate, size, expiration), nil
	}

	// Create the default limiter
	defaultLimiter, err := createLimiter(cfg.Default)
	if err != nil {
		return nil, fmt.Errorf("failed to create default rate limiter: %w", err)
	}
	limiters[defaultPolicyName] = defaultLimiter

	// Create limiters for other named policies
	for name, policy := range cfg.Policies {
		limiter, err := createLimiter(policy)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy '%s': %w", name, err)
		}
		limiters[name] = limiter
	}

	return &Manager{limiters: limiters}, nil
}

// Get retrieves a named rate limiter.
// If a limiter with the given name is not found, it returns the default limiter.
func (m *Manager) Get(name string) *RedisRateLimiter {
	if limiter, ok := m.limiters[name]; ok {
		return limiter
	}
	// Fallback to the default limiter, which is guaranteed to exist by the constructor.
	return m.limiters[defaultPolicyName]
}
