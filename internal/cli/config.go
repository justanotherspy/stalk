package cli

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/justanotherspy/stalk/internal/config"
)

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
		RunE: runConfigCheck,
	}
}

func runConfigCheck(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	v := viper.GetViper()

	if f := v.ConfigFileUsed(); f != "" {
		fmt.Fprintf(out, "config file: %s\n", f)
	} else {
		fmt.Fprintln(out, "config file: none found (using built-in defaults)")
	}

	cfg, err := config.Load(v)
	if err != nil {
		return err
	}

	socket, err := config.SocketPath()
	if err != nil {
		return err
	}
	db, err := config.DBPath()
	if err != nil {
		return err
	}

	d := cfg.Daemon
	fmt.Fprintln(out, "daemon:")
	fmt.Fprintf(out, "  idle_exit_after: %s\n", config.FormatDuration(d.IdleExitAfter))
	fmt.Fprintf(out, "  disconnect_grace: %s\n", config.FormatDuration(d.DisconnectGrace))
	fmt.Fprintf(out, "  max_subscriptions_per_session: %d\n", d.MaxSubscriptionsPerSession)
	fmt.Fprintf(out, "  max_connections: %d\n", d.MaxConnections)
	fmt.Fprintf(out, "  retention.events_max_age: %s\n", config.FormatDuration(d.Retention.EventsMaxAge))
	fmt.Fprintf(out, "  retention.events_max_rows: %d\n", d.Retention.EventsMaxRows)
	fmt.Fprintln(out, "paths:")
	fmt.Fprintf(out, "  socket: %s\n", socket)
	fmt.Fprintf(out, "  database: %s\n", db)

	if len(cfg.Sources) == 0 {
		fmt.Fprintln(out, "sources: none configured")
		fmt.Fprintln(out, "ok")
		return nil
	}

	fmt.Fprintf(out, "sources (%d):\n", len(cfg.Sources))
	for _, name := range slices.Sorted(maps.Keys(cfg.Sources)) {
		src := cfg.Sources[name]
		mode, err := config.ResolveAuthMode(&src, os.LookupEnv, func() bool {
			return ghTokenAvailable(cmd.Context())
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "  %s: %s auth=%s", name, src.Type, mode)
		switch src.Type {
		case config.SourceGitHub:
			if src.APIURL != "" {
				fmt.Fprintf(out, " api_url=%s", src.APIURL)
			}
			if len(src.AllowedOwners) > 0 {
				fmt.Fprintf(out, " allowed_owners=%s", strings.Join(src.AllowedOwners, ","))
			}
		case config.SourceHTTPPoll:
			fmt.Fprintf(out, " interval=%s url=%s", config.FormatDuration(src.Interval), src.URL)
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "ok: %d sources\n", len(cfg.Sources))
	return nil
}

// ghTokenAvailable reports whether the gh CLI can supply a token for the
// `gh auth` fallback chain. The token itself is only length-checked and never
// stored, logged, or printed.
func ghTokenAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	token, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	return err == nil && len(bytes.TrimSpace(token)) > 0
}
