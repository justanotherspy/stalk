package examples

import "testing"

// keySink defends against the compiler optimizing the benchmarked call away by
// giving the result an observable destination.
var keySink string

// BenchmarkNormalizeKey measures NormalizeKey using the modern b.Loop form
// (Go 1.24+). b.Loop runs the body the right number of times, keeps per-call
// setup out of the measured region, and prevents dead-code elimination of the
// call — so it is preferred over the classic `for range b.N` loop.
//
// Run it (and capture profiles) with:
//
//	make bench BENCH=BenchmarkNormalizeKey
//	make profile BENCH=BenchmarkNormalizeKey PROFPKG=./internal/examples
//	make pprof-cpu        # opens the flame graph in the pprof web UI
//
// Compare two revisions with benchstat:
//
//	git stash && make bench-save BENCHFILE=bench-old.txt && git stash pop
//	make bench-save BENCHFILE=bench-new.txt
//	make benchstat
func BenchmarkNormalizeKey(b *testing.B) {
	const input = "Some Config Key: with-mixed/Characters_123!!"
	b.ReportAllocs()
	for b.Loop() {
		keySink = NormalizeKey(input)
	}
}
