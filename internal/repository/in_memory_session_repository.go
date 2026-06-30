package repository

import (
	"context"
	"sync"
	"time"
)

type InMemorySessionRepository struct {
	mu       sync.Mutex
	sessions map[string]time.Time
	ttl      time.Duration
}

func NewInMemorySessionRepository(sessionTTL time.Duration) *InMemorySessionRepository {
	return &InMemorySessionRepository{sessions: make(map[string]time.Time), ttl: sessionTTL}
}

func (r *InMemorySessionRepository) Create(_ context.Context, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[token] = time.Now().Add(r.ttl)
	return nil
}

func (r *InMemorySessionRepository) Validate(_ context.Context, token string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	expiresAt, ok := r.sessions[token]
	if !ok {
		return false, nil
	}

	if time.Now().After(expiresAt) {
		delete(r.sessions, token)
		return false, nil
	}

	return true, nil
}

func (r *InMemorySessionRepository) Revoke(_ context.Context, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, token)
	return nil
}
