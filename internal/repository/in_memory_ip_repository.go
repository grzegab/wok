package repository

import (
	"context"
	"time"
)

type InMemoryIpRepository struct {
	ips map[string]string
}

func NewInMemoryIpRepository() *InMemoryIpRepository {
	return &InMemoryIpRepository{ips: make(map[string]string)}
}

func (r *InMemoryIpRepository) Add(_ context.Context, ip string, reason string) error {
	r.ips[ip] = reason
	return nil
}

func (r *InMemoryIpRepository) Exists(ctx context.Context, ip string) (bool, error) {
	_, ok := r.ips[ip]
	if ok {
		select {
		case <-ctx.Done():
		case <-time.After(200 * time.Millisecond):
		}
	}
	return ok, nil
}
