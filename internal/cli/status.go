package cli

import "github.com/spf13/cobra"

// newStatusCmd returns the status command: a one-shot introspection client for
// a running daemon.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon, session, and subscription status",
		Long: `Report on the running daemon.

Prints its uptime and instance id, the attached sessions, every active
subscription, and per-source poll health: consecutive errors, current backoff,
and the last HTTP status seen. Reports cleanly when no daemon is running rather
than starting one.`,
		Args: cobra.NoArgs,
		RunE: notImplemented,
	}
}
