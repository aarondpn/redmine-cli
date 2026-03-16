package tracker

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewCmdTrackers creates the trackers command group.
func NewCmdTrackers(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trackers",
		Short: "Manage trackers",
	}

	cmd.AddCommand(newCmdTrackerList(f))
	return cmd
}

func newCmdTrackerList(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all trackers",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching trackers...")
			trackers, err := client.Trackers.List(context.Background())
			stop()
			if err != nil {
				return err
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(trackers)
			case output.FormatCSV:
				headers := []string{"ID", "Name", "Description"}
				rows := make([][]string, len(trackers))
				for i, t := range trackers {
					rows[i] = []string{fmt.Sprintf("%d", t.ID), t.Name, t.Description}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "Name", "Description"}
				rows := make([][]string, len(trackers))
				for i, t := range trackers {
					rows[i] = []string{
						output.StyleID.Render(fmt.Sprintf("%d", t.ID)),
						t.Name,
						t.Description,
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
