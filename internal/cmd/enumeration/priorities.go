package enumeration

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

// NewCmdPriorities creates the priorities command group.
func NewCmdPriorities(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "priorities",
		Aliases: []string{"priority"},
		Short:   "List issue priorities",
	}
	cmd.AddCommand(newCmdPrioritiesList(f))
	return cmd
}

func newCmdPrioritiesList(f *cmdutil.Factory) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List all issue priorities",
		Example: `  # List priorities with their IDs
  redmine priorities list

  # As JSON
  redmine priorities list -o json

  # ID of the default priority
  redmine priorities list --jq '.[] | select(.is_default) | .id'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)
			stop := printer.Spinner("Fetching priorities...")
			result, err := ops.ListPriorities(cmdutil.Context(cmd), client, struct{}{})
			stop()
			if err != nil {
				return err
			}
			renderEnumerations(printer, result.Priorities, "priorities", true)
			return nil
		},
	}
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
