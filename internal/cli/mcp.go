package cli

import "github.com/spf13/cobra"

// newMCPCmd returns the mcp command: the per-session MCP server that gives the
// agent subscribe and unsubscribe control over the daemon.
func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the stalk MCP server on stdio",
		Long: `Run the stalk MCP server on stdio.

Exposes subscribe_event, unsubscribe_event, list_subscriptions, list_sources,
and get_event to the agent, forwarding each to the daemon over the socket. The
agent decides what to watch; the daemon does the watching.

Event payloads reach the agent as untrusted data and are labeled as such in
the tool descriptions. Logs go to stderr so stdout stays a clean protocol
channel.`,
		Args: cobra.NoArgs,
		RunE: notImplemented,
	}
}
