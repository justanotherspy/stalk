package examples

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// TestPollUntil_succeeds shows the testing/synctest pattern (stable since Go
// 1.25; the older synctest.Run was removed in 1.26 — always use synctest.Test).
//
// synctest.Test runs the body inside a "bubble": every goroutine started from
// it shares an isolated, fake clock that only advances when all goroutines are
// durably blocked. That makes time-dependent concurrency deterministic and
// instant — no real sleeping, no flakes.
//
// synctest.Wait blocks until every other goroutine in the bubble is durably
// blocked, giving a precise point to assert state or advance the clock.
func TestPollUntil_succeeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var ready atomic.Bool
		errc := make(chan error, 1)
		go func() {
			errc <- PollUntil(context.Background(), time.Second, ready.Load)
		}()

		// The poller is now blocked on its ticker against the fake clock.
		synctest.Wait()
		select {
		case err := <-errc:
			t.Fatalf("PollUntil returned early: %v", err)
		default:
		}

		// Satisfy the condition, then let the fake clock advance one tick: this
		// is instantaneous in wall-clock terms.
		ready.Store(true)
		time.Sleep(time.Second)
		synctest.Wait()

		if err := <-errc; err != nil {
			t.Fatalf("PollUntil: %v", err)
		}
	})
}

// TestPollUntil_timeout exercises the cancellation path. A 5s context timeout
// that would slow a normal test runs in microseconds inside the bubble.
func TestPollUntil_timeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := PollUntil(ctx, time.Second, func() bool { return false })
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("PollUntil err = %v, want context.DeadlineExceeded", err)
		}
	})
}
