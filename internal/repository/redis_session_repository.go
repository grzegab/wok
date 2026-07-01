package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSessionRepository struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisSessionRepository(rdb *redis.Client, ttl time.Duration) *RedisSessionRepository {
	return &RedisSessionRepository{rdb: rdb, ttl: ttl}
}

func (r *RedisSessionRepository) Create(ctx context.Context, token string) error {
	if err := r.rdb.Set(ctx, "session:"+token, "1", r.ttl).Err(); err != nil {
		return fmt.Errorf("could not create session: %w", err)
	}
	return nil
}

func (r *RedisSessionRepository) Validate(ctx context.Context, token string) (bool, error) {
	val, err := r.rdb.Exists(ctx, "session:"+token).Result()
	if err != nil {
		return false, fmt.Errorf("could not validate session: %w", err)
	}
	return val > 0, nil
}

func (r *RedisSessionRepository) Revoke(ctx context.Context, token string) error {
	if err := r.rdb.Del(ctx, "session:"+token).Err(); err != nil {
		return fmt.Errorf("could not revoke session: %w", err)
	}
	return nil
}
