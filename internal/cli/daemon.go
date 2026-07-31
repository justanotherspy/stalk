package cli

import "github.com/spf13/cobra"

// newDaemonCmd returns the daemon command: the single per-user process that
// owns the socket, the SQLite state, and every poll loop.
func newDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the stalk daemon",
		Long: `Run the per-user stalk daemon.

The daemon binds the unix socket, opens the SQLite state database, polls every
configured source, and fans events out to attached Claude Code sessions. It is
a singleton per user, is auto-spawned by the first client that needs it, and
exits once it has been idle for daemon.idle_exit_after.

See docs/PROTOCOL.md for the socket and wire protocol.`,
		Args: cobra.NoArgs,
		RunE: notImplemented,
	}
}
