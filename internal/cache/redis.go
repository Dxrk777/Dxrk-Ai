// Package cache provides Redis-backed caching utilities.
package cache

import (
	"github.com/redis/go-redis/v9"
)

// Options re-exports redis options for convenience.
type Options = redis.Options

// NewClient creates a new Redis client with the given options.
func NewClient(opts *redis.Options) *redis.Client {
	return redis.NewClient(opts)
}
