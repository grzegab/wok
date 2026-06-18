package repository

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const banTime = 30 * time.Minute

type RedisIpRepository struct {
	rdb *redis.Client
}

func NewRedisIpRepository(rdb *redis.Client) *RedisIpRepository {
	return &RedisIpRepository{rdb: rdb}
}

func (r *RedisIpRepository) Add(ctx context.Context, ip string, reason string) error {
	return r.rdb.Set(ctx, "banned_ip:"+ip, reason, banTime).Err()
}

func (r *RedisIpRepository) Exists(ctx context.Context, ip string) (bool, error) {
	val, err := r.rdb.Exists(ctx, "banned_ip:"+ip).Result()
	return val > 0, err
}
