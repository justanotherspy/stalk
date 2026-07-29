// Package examples contains small, self-contained reference implementations
// that demonstrate this repository's testing and profiling conventions:
//
//   - fuzzing a pure function (see FuzzNormalizeKey in fuzz_test.go),
//   - benchmarking + profiling with the modern b.Loop loop (bench_test.go),
//   - testing concurrent, time-dependent code deterministically with
//     testing/synctest (synctest_test.go).
//
// It is reference material for the template. When you build your own packages,
// replace or delete this one — nothing else imports it.
package examples

import (
	"context"
	"strings"
	"time"
)

// NormalizeKey converts an arbitrary string into a canonical configuration /
// environment-variable key: ASCII letters are upper-cased, runs of any other
// characters collapse to a single underscore, and leading/trailing underscores
// are trimmed. The result therefore matches ^([A-Z0-9]+(_[A-Z0-9]+)*)?$ and is
// idempotent — properties the fuzz test asserts.
//
// It pairs naturally with Viper's GO_TEMPLATE_ env-var prefix: NormalizeKey
// turns a human label like "Max Retry Count" into "MAX_RETRY_COUNT".
func NormalizeKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSep := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			r -= 'a' - 'A'
			fallthrough
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
		default:
			// Defer the separator: it is only emitted if another keep-able
			// rune follows, which avoids any trailing underscore.
			pendingSep = true
		}
	}
	return b.String()
}

// PollUntil calls check every interval until it reports true, returning nil.
// If ctx is canceled first it returns ctx.Err(). check is evaluated once up
// front before any waiting.
//
// It uses a time.Ticker and select on ctx.Done, which makes it both genuinely
// useful and a good showcase for testing/synctest: under a synctest bubble the
// ticker fires against a fake clock, so a test exercises the timeout path in
// microseconds instead of real seconds (see synctest_test.go).
func PollUntil(ctx context.Context, interval time.Duration, check func() bool) error {
	if check() {
		return nil
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if check() {
				return nil
			}
		}
	}
}
