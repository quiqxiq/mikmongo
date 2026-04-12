// Package redis provides Redis client wrapper
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps redis.Client
type Client struct {
	client *redis.Client
}

// Options contains Redis connection options
type Options struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// NewClient creates a new Redis client
func NewClient(opts Options) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Password: opts.Password,
		DB:       opts.DB,
	})
	return &Client{client: rdb}
}

// Ping checks Redis connection
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the connection
func (c *Client) Close() error {
	return c.client.Close()
}

// HealthCheck performs health check
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return c.client.Ping(ctx).Err()
}

// GoRedis returns the underlying go-redis client for direct operations
// such as HGETALL, XREVRANGE, etc.
func (c *Client) GoRedis() *redis.Client {
	return c.client
}

// FlushDB removes all keys from the current database. Use only in tests.
func (c *Client) FlushDB(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}
