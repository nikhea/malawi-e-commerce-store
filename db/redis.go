package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient parses REDIS_URL (falling back to host/port/db fields),
// connects, and pings. Rate limiting fails OPEN on Redis errors at
// request time (the shop stays up), but a dead Redis at boot is still a
// loud startup warning — callers decide whether to fatal.
func NewRedisClient(ctx context.Context, url, host string, port, db int, password string) (*redis.Client, error) {
	opt := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	}
	if url != "" {
		parsed, err := redis.ParseURL(url)
		if err != nil {
			return nil, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		opt = parsed
	}

	client := redis.NewClient(opt)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}
