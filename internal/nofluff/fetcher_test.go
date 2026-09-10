package nofluff

import (
	"context"
	"testing"
	"time"
)

func TestFetchOffers(t *testing.T) {
	// ...
}

func TestPause(t *testing.T) {
	tests := []struct {
		name       string
		wait       time.Duration
		arrange    func(cancel context.CancelFunc)
		want       bool
		maxElapsed time.Duration
	}{
		{
			name:       "waits out the delay and reports it finished",
			wait:       time.Millisecond,
			want:       true,
			maxElapsed: time.Second,
		},
		{
			name:       "an already cancelled context returns at once",
			wait:       time.Hour,
			arrange:    func(cancel context.CancelFunc) { cancel() },
			want:       false,
			maxElapsed: 100 * time.Millisecond,
		},
		{
			name:       "cancelling mid-wait cuts the delay short",
			wait:       time.Hour,
			arrange:    func(cancel context.CancelFunc) { time.AfterFunc(10*time.Millisecond, cancel) },
			want:       false,
			maxElapsed: time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.arrange != nil {
				tt.arrange(cancel)
			}

			start := time.Now()
			got := pause(ctx, tt.wait)
			elapsed := time.Since(start)

			if got != tt.want {
				t.Errorf("pause() got = %v, want %v", got, tt.want)
			}

			if elapsed > tt.maxElapsed {
				t.Errorf("pause() elapsed = %v, want %v", elapsed, tt.maxElapsed)
			}
		})
	}
}
