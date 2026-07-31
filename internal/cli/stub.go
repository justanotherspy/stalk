package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// notImplemented is the RunE body shared by every command that is wired into
// the tree but not built yet. Returning an error makes Execute print it to
// stderr and exit non-zero (see Execute), so a stub is never mistaken for a
// command that quietly did nothing.
//
// Each stub loses this line in the PR that implements it; when the last one
// does, this file goes away. See docs/MVP-SPEC.md §7 for the milestones.
func notImplemented(cmd *cobra.Command, _ []string) error {
	return fmt.Errorf("%s: not implemented (see docs/MVP-SPEC.md for the milestone that lands it)", cmd.CommandPath())
}
