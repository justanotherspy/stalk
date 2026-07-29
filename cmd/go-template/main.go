// Command go-template is the entrypoint for the CLI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/justanotherspy/go-template/internal/cli"
)

// Build information. Populated at release time via -ldflags by GoReleaser
// (see .goreleaser.yaml) and by the Makefile `build` target.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Cancel the root context on Ctrl-C / SIGTERM so long-running commands can
	// shut down cleanly; stop() restores the default signal behavior.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cli.Execute(ctx, cli.BuildInfo{Version: version, Commit: commit, Date: date})
}
