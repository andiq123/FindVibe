package providers

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPacerMinGap(t *testing.T) {
	p := newPacer(40 * time.Millisecond)
	ctx := context.Background()
	start := time.Now()
	if err := p.wait(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.wait(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 35*time.Millisecond {
		t.Fatalf("expected gap, got %v", elapsed)
	}
}

func TestPacerDoesNotHoldLockWhileSleeping(t *testing.T) {
	p := newPacer(80 * time.Millisecond)
	ctx := context.Background()
	_ = p.wait(ctx) // arm last

	var wg sync.WaitGroup
	started := make(chan struct{}, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started <- struct{}{}
			_ = p.wait(ctx)
		}()
	}

	// Both goroutines should enter wait (and one start sleeping) without serializing on the mutex for the full gap.
	deadline := time.After(30 * time.Millisecond)
	for range 2 {
		select {
		case <-started:
		case <-deadline:
			t.Fatal("callers blocked on mutex while another slept")
		}
	}
	wg.Wait()
}
