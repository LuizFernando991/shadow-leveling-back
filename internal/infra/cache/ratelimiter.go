package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter checks whether an action identified by key is within the allowed
// limit for the given time window. Implementations must be safe for concurrent use.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

type NoopRateLimiter struct{}

func (NoopRateLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return true, nil
}

type redisRateLimiter struct {
	client *redis.Client
}

func NewRedisRateLimiter(client *redis.Client) RateLimiter {
	return &redisRateLimiter{client: client}
}

func NewRedisClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func (r *redisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("cache: rate limit incr: %w", err)
	}
	// Set the expiry only on the first increment so the window is fixed, not sliding.
	if count == 1 {
		if err := r.client.Expire(ctx, key, window).Err(); err != nil {
			return false, fmt.Errorf("cache: rate limit expire: %w", err)
		}
	}
	return count <= int64(limit), nil
}
