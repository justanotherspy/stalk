package cli

import "github.com/spf13/cobra"

// newStreamCmd returns the stream command: the per-session event channel that
// the Claude Code plugin runs as a monitor.
func newStreamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stream",
		Short: "Stream this session's events to stdout",
		Long: `Stream events for the current Claude Code session.

Connects to the daemon (spawning it if needed), registers as this session's
stream, and prints one plain-text line per delivered event to stdout. Each line
is acknowledged back to the daemon only after it has been written, which is
what makes delivery at-least-once.

Run as a plugin monitor, every line becomes a Claude Code notification. Logs go
to stderr so stdout stays a clean protocol channel.`,
		Args: cobra.NoArgs,
		RunE: notImplemented,
	}
}
