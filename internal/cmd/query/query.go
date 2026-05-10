package query

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdQueries creates the queries command group.
func NewCmdQueries(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "queries",
		Aliases: []string{"q"},
		Short:   "Manage saved queries",
		Long:    "List Redmine saved queries (custom filters). Use the resulting ID with `issues list --query-id` to run a saved query from the CLI.",
	}

	cmd.AddCommand(newCmdQueryList(f))
	cmd.AddCommand(newCmdQueryGet(f))

	return cmd
}
