package examples

import (
	"regexp"
	"testing"
)

// keyShape is the canonical form NormalizeKey promises to return.
var keyShape = regexp.MustCompile(`^([A-Z0-9]+(_[A-Z0-9]+)*)?$`)

// FuzzNormalizeKey checks the invariants of NormalizeKey against arbitrary
// input rather than fixed examples:
//
//   - the output always matches the canonical key shape, and
//   - NormalizeKey is idempotent: normalizing an already-normalized key is a
//     no-op.
//
// The f.Add calls form the seed corpus. `go test` (and therefore `make test`)
// runs the seeds as a normal unit test, so the invariants are checked in CI on
// every change; `make fuzz FUZZ=FuzzNormalizeKey` mutates from the seeds to
// hunt for new failing inputs, and the nightly fuzz workflow does the same on a
// schedule. Any crasher is written to testdata/fuzz/ — commit it as a
// regression seed.
func FuzzNormalizeKey(f *testing.F) {
	for _, s := range []string{"", "Max Retry Count", "--flag.name--", "  ", "ünïcödé 123", "ALREADY_OK"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := NormalizeKey(s)
		if !keyShape.MatchString(got) {
			t.Fatalf("NormalizeKey(%q) = %q, which is not a canonical key", s, got)
		}
		if again := NormalizeKey(got); again != got {
			t.Fatalf("NormalizeKey not idempotent: NormalizeKey(%q) = %q", got, again)
		}
	})
}
