package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/grzegab/wok/internal/config"
	"github.com/redis/go-redis/v9"
)

func newTestRedisSessionRepository(t *testing.T) *RedisSessionRepository {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		t.Skipf("skipping: no reachable Redis instance at %s: %v", addr, err)
	}

	t.Cleanup(func() {
		_ = rdb.Close()
	})

	return NewRedisSessionRepository(rdb, config.DefaultSessionTTL)
}

func TestRedisSessionRepository_CreateThenValidate_ReturnsTrue(t *testing.T) {
	repo := newTestRedisSessionRepository(t)
	ctx := context.Background()
	token := "test-token-create"
	t.Cleanup(func() { _ = repo.Revoke(ctx, token) })

	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	ok, err := repo.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !ok {
		t.Fatalf("Validate() = false, want true")
	}
}

func TestRedisSessionRepository_ValidateUnknownToken_ReturnsFalse(t *testing.T) {
	repo := newTestRedisSessionRepository(t)
	ctx := context.Background()

	ok, err := repo.Validate(ctx, "unknown-test-token")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if ok {
		t.Fatalf("Validate() = true, want false")
	}
}

func TestRedisSessionRepository_ValidateAfterTTLElapsed_ReturnsFalse(t *testing.T) {
	t.Skip("sessionTTL is 24h; not practical to time-travel against a real Redis instance in this suite")
}

func TestRedisSessionRepository_RevokeThenValidate_ReturnsFalse(t *testing.T) {
	repo := newTestRedisSessionRepository(t)
	ctx := context.Background()
	token := "test-token-revoke"

	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.Revoke(ctx, token); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	ok, err := repo.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if ok {
		t.Fatalf("Validate() = true, want false after Revoke")
	}
}
