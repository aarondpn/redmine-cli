package status

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/spf13/cobra"
)

// NewCmdStatuses creates the statuses command group.
func NewCmdStatuses(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "statuses",
		Short: "Manage issue statuses",
	}

	cmd.AddCommand(newCmdStatusList(f))
	return cmd
}

func newCmdStatusList(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Short:   "List all issue statuses",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching statuses...")
			result, err := ops.ListStatuses(cmdutil.Context(cmd), client, struct{}{})
			stop()
			if err != nil {
				return err
			}
			statuses := result.Statuses

			cmdutil.RenderCollection(printer, statuses, []string{"ID", "Name", "Closed"}, func(s models.IssueStatus) []string {
				closed := "no"
				if s.IsClosed {
					closed = "yes"
				}
				return []string{
					output.StyleID.Render(fmt.Sprintf("%d", s.ID)),
					output.StatusStyle(s.Name).Render(s.Name),
					closed,
				}
			})
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
