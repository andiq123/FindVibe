package providers

import (
	"context"
	"sync"
	"time"
)

// pacer serializes bursts so parallel explore/radio resolves don't stampede a provider.
type pacer struct {
	mu     sync.Mutex
	last   time.Time
	minGap time.Duration
}

func newPacer(minGap time.Duration) *pacer {
	return &pacer{minGap: minGap}
}

// wait sleeps outside the mutex so other callers can queue without blocking the lock.
func (p *pacer) wait(ctx context.Context) error {
	p.mu.Lock()
	var wait time.Duration
	if !p.last.IsZero() {
		if d := p.minGap - time.Since(p.last); d > 0 {
			wait = d
		}
	}
	// Reserve this slot so the next waiter measures from our planned start.
	p.last = time.Now().Add(wait)
	p.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
