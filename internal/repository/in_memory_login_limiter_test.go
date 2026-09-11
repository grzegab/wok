package repository

import (
	"testing"
	"time"
)

func TestInMemoryLoginLimiter_BlocksAfterMaxAttempts(t *testing.T) {
	limiter := NewInMemoryLoginLimiter(3, time.Minute)

	for i := range 3 {
		if !limiter.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if limiter.Allow("1.2.3.4") {
		t.Fatal("attempt over the limit should be blocked")
	}
	if !limiter.Allow("5.6.7.8") {
		t.Fatal("another IP should not be affected")
	}
}

func TestInMemoryLoginLimiter_AllowsAgainAfterWindow(t *testing.T) {
	limiter := NewInMemoryLoginLimiter(1, 10*time.Millisecond)

	limiter.Allow("1.2.3.4")
	if limiter.Allow("1.2.3.4") {
		t.Fatal("second attempt in the window should be blocked")
	}

	time.Sleep(20 * time.Millisecond)

	if !limiter.Allow("1.2.3.4") {
		t.Fatal("attempt after the window should be allowed")
	}
}

func TestInMemoryLoginLimiter_ResetClearsAttempts(t *testing.T) {
	limiter := NewInMemoryLoginLimiter(1, time.Minute)

	limiter.Allow("1.2.3.4")
	limiter.Reset("1.2.3.4")

	if !limiter.Allow("1.2.3.4") {
		t.Fatal("attempt after reset should be allowed")
	}
}
