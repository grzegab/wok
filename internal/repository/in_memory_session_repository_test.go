package repository

import (
	"context"
	"testing"
	"time"
)

const ttl = 1 * time.Second

func TestInMemorySessionRepository_CreateThenValidate_ReturnsTrue(t *testing.T) {
	repo := NewInMemorySessionRepository(ttl)
	ctx := context.Background()

	if err := repo.Create(ctx, "token-1"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	ok, err := repo.Validate(ctx, "token-1")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !ok {
		t.Fatalf("Validate() = false, want true")
	}
}

func TestInMemorySessionRepository_ValidateUnknownToken_ReturnsFalse(t *testing.T) {
	repo := NewInMemorySessionRepository(ttl)
	ctx := context.Background()

	ok, err := repo.Validate(ctx, "unknown-token")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if ok {
		t.Fatalf("Validate() = true, want false")
	}
}

func TestInMemorySessionRepository_ValidateAfterTTLElapsed_ReturnsFalse(t *testing.T) {
	repo := NewInMemorySessionRepository(ttl)
	repo.ttl = 10 * time.Millisecond
	ctx := context.Background()

	if err := repo.Create(ctx, "token-2"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	ok, err := repo.Validate(ctx, "token-2")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if ok {
		t.Fatalf("Validate() = true, want false after TTL elapsed")
	}
}

func TestInMemorySessionRepository_RevokeThenValidate_ReturnsFalse(t *testing.T) {
	repo := NewInMemorySessionRepository(ttl)
	ctx := context.Background()

	if err := repo.Create(ctx, "token-3"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.Revoke(ctx, "token-3"); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	ok, err := repo.Validate(ctx, "token-3")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if ok {
		t.Fatalf("Validate() = true, want false after Revoke")
	}
}
