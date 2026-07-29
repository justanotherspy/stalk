package cli_test

// End-to-end tests for the CLI.
//
// Unlike root_test.go (which drives the cobra command in-process via buffers),
// these exercise the program the way a user does: as a real command with
// arguments, stdout/stderr, exit codes, and environment variables. We use
// rogpeppe/go-internal/testscript, the same harness the Go toolchain uses for
// its own CLI tests. Each case is a `.txtar` script under testdata/script/.
//
// testscript.RunMain re-execs THIS test binary for every `go-template` command
// in a script, dispatching to the function registered below — so there is no
// separate build step and coverage still attributes to the cli package.

import (
	"context"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/justanotherspy/go-template/internal/cli"
)

// TestMain lets the test binary masquerade as the `go-template` command when
// invoked by testscript, and otherwise runs the normal test suite.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"go-template": func() {
			// cli.Execute calls os.Exit itself (non-zero on error); on success
			// it returns and testscript records a clean exit.
			cli.Execute(context.Background(), cli.BuildInfo{
				Version: "e2e",
				Commit:  "testcommit",
				Date:    "2026-01-01",
			})
		},
	})
}

// TestScripts runs every testdata/script/*.txtar scenario.
func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
	})
}
