// Package cli wires up the command tree for the application.
package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// BuildInfo carries version metadata injected at build time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

var (
	buildInfo BuildInfo
	cfgFile   string
	verbose   bool
	logLevel  string
	logFormat string
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "go-template",
		Short:         "A starter Go CLI",
		Long:          "go-template is a starter template for building Go command-line applications.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := initConfig(cmd); err != nil {
				return err
			}

			// Build the logger from the resolved config and make it the default,
			// and stash it on the command context so subcommands share it.
			logger := setupLogging(cmd.ErrOrStderr())
			slog.SetDefault(logger)
			cmd.SetContext(withLogger(cmd.Context(), logger))

			logger.Debug("configuration loaded",
				"config_file", viper.ConfigFileUsed(),
				"log_level", viper.GetString("log.level"),
				"log_format", viper.GetString("log.format"),
			)
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	pf := cmd.PersistentFlags()
	pf.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.go-template.yaml)")
	pf.BoolVarP(&verbose, "verbose", "v", false, "enable verbose (debug) logging")
	pf.StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	pf.StringVar(&logFormat, "log-format", "text", "log format: text or json")

	cmd.AddCommand(newVersionCmd())

	return cmd
}

// NewRootCmd returns the fully wired root command. It is exported for
// documentation generation (see cmd/gen-docs, which renders shell completions
// and man pages); use Execute to actually run the CLI.
func NewRootCmd() *cobra.Command {
	return newRootCmd()
}

// initConfig loads configuration from a file (if present) and the environment,
// then binds the persistent flags so that flags, GO_TEMPLATE_ environment
// variables, and the config file all feed the same viper keys (in that order of
// precedence). Environment variables are read with the GO_TEMPLATE_ prefix;
// nested keys map with "." and "-" replaced by "_" (e.g. log.level ->
// GO_TEMPLATE_LOG_LEVEL).
func initConfig(cmd *cobra.Command) error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".go-template")
	}

	viper.SetEnvPrefix("GO_TEMPLATE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	// Let flags win over env/config when explicitly set.
	_ = viper.BindPFlag("verbose", cmd.Flags().Lookup("verbose"))
	_ = viper.BindPFlag("log.level", cmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("log.format", cmd.Flags().Lookup("log-format"))

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return err
		}
	}

	return nil
}

// Execute builds the root command and runs it under ctx (which carries
// OS-signal cancellation from main), exiting non-zero on error.
func Execute(ctx context.Context, info BuildInfo) {
	buildInfo = info
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "aborted")
		} else {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(1)
	}
}
