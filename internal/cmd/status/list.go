package status

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
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
		Short:   "List all issue statuses",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching statuses...")
			statuses, err := client.Statuses.List(context.Background())
			stop()
			if err != nil {
				return err
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(statuses)
			case output.FormatCSV:
				headers := []string{"ID", "Name", "Closed"}
				rows := make([][]string, len(statuses))
				for i, s := range statuses {
					closed := "no"
					if s.IsClosed {
						closed = "yes"
					}
					rows[i] = []string{fmt.Sprintf("%d", s.ID), s.Name, closed}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "Name", "Closed"}
				rows := make([][]string, len(statuses))
				for i, s := range statuses {
					closed := "no"
					if s.IsClosed {
						closed = "yes"
					}
					rows[i] = []string{
						output.StyleID.Render(fmt.Sprintf("%d", s.ID)),
						output.StatusStyle(s.Name).Render(s.Name),
						closed,
					}
				}
				printer.Table(headers, rows)
			}
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
