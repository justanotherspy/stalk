package cli

import "github.com/spf13/cobra"

// newConfigCmd returns the config command group. The group itself only prints
// help; Args is set so that an unknown subcommand fails instead of silently
// printing help and exiting zero (cobra skips argument validation entirely for
// a command that is not runnable).
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the stalk configuration",
		Long: `Inspect the stalk configuration.

Configuration is resolved from flags, then STALK_ environment variables, then
the config file (--config, default $HOME/.stalk.yaml), then built-in defaults.
See .stalk.yaml.example for the full set of keys.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newConfigCheckCmd())

	return cmd
}

// newConfigCheckCmd returns the config check command: validation of the
// resolved configuration, without contacting a daemon or any source.
func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate the resolved configuration",
		Long: `Validate the resolved configuration and report what stalk would do with it.

Checks that every source is well formed, that intervals are within their
allowed range, and that each token_var names an environment variable that is
actually set. Reports which file was loaded and exits non-zero on the first
problem. Never prints token values, only whether a credential resolved.`,
		Args: cobra.NoArgs,
		RunE: notImplemented,
	}
}
