package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loggerFrom(cmd.Context()).Debug("printing version information", "version", buildInfo.Version)

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "go-template %s\n", buildInfo.Version)
			fmt.Fprintf(out, "  commit:  %s\n", buildInfo.Commit)
			fmt.Fprintf(out, "  built:   %s\n", buildInfo.Date)
			fmt.Fprintf(out, "  go:      %s\n", runtime.Version())
			fmt.Fprintf(out, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
