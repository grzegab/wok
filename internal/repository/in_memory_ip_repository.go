package repository

import (
	"context"
	"sync"
	"time"
)

type InMemoryIpRepository struct {
	mu       sync.Mutex
	bannedAt map[string]time.Time
	ttl      time.Duration
}

func NewInMemoryIpRepository() *InMemoryIpRepository {
	return &InMemoryIpRepository{bannedAt: make(map[string]time.Time), ttl: banTime}
}

func (r *InMemoryIpRepository) Add(_ context.Context, ip string, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for bannedIp, at := range r.bannedAt {
		if now.Sub(at) >= r.ttl {
			delete(r.bannedAt, bannedIp)
		}
	}
	r.bannedAt[ip] = now
	return nil
}

func (r *InMemoryIpRepository) Exists(ctx context.Context, ip string) (bool, error) {
	r.mu.Lock()
	at, ok := r.bannedAt[ip]
	r.mu.Unlock()

	banned := ok && time.Since(at) < r.ttl
	if banned {
		select {
		case <-ctx.Done():
		case <-time.After(200 * time.Millisecond):
		}
	}
	return banned, nil
}
