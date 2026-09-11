package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestInMemoryIpRepository_BanExpiresAfterTTL(t *testing.T) {
	repo := NewInMemoryIpRepository()
	repo.ttl = 10 * time.Millisecond
	ctx := context.Background()

	repo.Add(ctx, "1.2.3.4", "test")
	if banned, _ := repo.Exists(ctx, "1.2.3.4"); !banned {
		t.Fatal("IP should be banned right after Add")
	}

	time.Sleep(20 * time.Millisecond)

	if banned, _ := repo.Exists(ctx, "1.2.3.4"); banned {
		t.Fatal("ban should expire after TTL")
	}
}

func TestInMemoryIpRepository_ConcurrentAccessIsSafe(t *testing.T) {
	repo := NewInMemoryIpRepository()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var wg sync.WaitGroup
	for i := range 50 {
		ip := fmt.Sprintf("10.0.0.%d", i)
		wg.Go(func() { repo.Add(ctx, ip, "test") })
		wg.Go(func() { repo.Exists(ctx, ip) })
	}
	wg.Wait()
}
