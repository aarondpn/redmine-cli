package tracker

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
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

			cmdutil.RenderCollection(printer, trackers, []string{"ID", "Name", "Description"}, func(t models.Tracker, styled bool) []string {
				id := fmt.Sprintf("%d", t.ID)
				if styled {
					id = output.StyleID.Render(id)
				}
				return []string{id, t.Name, t.Description}
			})
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
