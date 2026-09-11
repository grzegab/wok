package repository

import (
	"sync"
	"time"
)

type loginWindow struct {
	attempts int
	start    time.Time
}

type InMemoryLoginLimiter struct {
	mu          sync.Mutex
	windows     map[string]loginWindow
	maxAttempts int
	window      time.Duration
}

func NewInMemoryLoginLimiter(maxAttempts int, window time.Duration) *InMemoryLoginLimiter {
	return &InMemoryLoginLimiter{windows: make(map[string]loginWindow), maxAttempts: maxAttempts, window: window}
}

func (l *InMemoryLoginLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for windowIp, w := range l.windows {
		if now.Sub(w.start) >= l.window {
			delete(l.windows, windowIp)
		}
	}

	w, ok := l.windows[ip]
	if !ok {
		w = loginWindow{start: now}
	}
	if w.attempts >= l.maxAttempts {
		return false
	}
	w.attempts++
	l.windows[ip] = w
	return true
}

func (l *InMemoryLoginLimiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.windows, ip)
}
