package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

type Client struct {
	rdb *redis.Client
}

func Connect(ctx context.Context, redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	rdb := redis.NewClient(opts)

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	slog.Info("Connected to Redis successfully")

	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() {
	if c.rdb != nil {
		c.rdb.Close()
		slog.Info("Redis connection closed")
	}
}

// Allow limits requests based on a token bucket algorithm.
// Returns nil if allowed, ErrRateLimitExceeded if not.
func (c *Client) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	// Simple Fixed Window / Token Bucket using Lua for atomicity
	script := `
		local current = redis.call('GET', KEYS[1])
		if current and tonumber(current) >= tonumber(ARGV[1]) then
			return 0
		end
		
		current = redis.call('INCR', KEYS[1])
		if tonumber(current) == 1 then
			redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
		end
		
		return 1
	`
	
	windowSeconds := int(window.Seconds())
	if windowSeconds <= 0 {
		windowSeconds = 1 // Prevent 0 expiry
	}

	res, err := c.rdb.Eval(ctx, script, []string{key}, limit, windowSeconds).Result()
	if err != nil {
		return fmt.Errorf("rate limiter script failed: %w", err)
	}

	allowed, ok := res.(int64)
	if !ok || allowed == 0 {
		return ErrRateLimitExceeded
	}

	return nil
}

// GetCache / SetCache methods for non-critical metadata
func (c *Client) SetCache(ctx context.Context, key string, value string, exp time.Duration) error {
	return c.rdb.Set(ctx, key, value, exp).Err()
}

func (c *Client) GetCache(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // cache miss
	}
	return val, err
}
