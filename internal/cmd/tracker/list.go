package tracker

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdTrackers creates the trackers command group.
func NewCmdTrackers(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trackers",
		Short: "Manage trackers",
		Long:  "List and inspect Redmine trackers and their supported standard fields.",
	}

	cmd.AddCommand(newCmdTrackerList(f))
	cmd.AddCommand(newCmdTrackerGet(f))
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
			result, err := ops.ListTrackers(context.Background(), client, struct{}{})
			stop()
			if err != nil {
				return err
			}
			trackers := result.Trackers

			cmdutil.RenderCollection(printer, trackers, []string{"ID", "Name", "Default Status", "Description"}, func(t models.Tracker, styled bool) []string {
				id := fmt.Sprintf("%d", t.ID)
				if styled {
					id = output.StyleID.Render(id)
				}
				return []string{id, t.Name, trackerDefaultStatus(t), t.Description}
			})
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func newCmdTrackerGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:               "get <id-or-name>",
		Aliases:           []string{"show", "view"},
		Short:             "Show tracker details",
		Long:              "Show tracker details. Accepts a numeric ID or tracker name.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cmdutil.CompleteTrackers(f),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			id, err := resolver.ResolveTracker(ctx, client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching tracker...")
			tracker, err := ops.GetTracker(ctx, client, ops.GetTrackerInput{ID: id})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(tracker)
				return nil
			}
			if printer.Format() == output.FormatCSV {
				printer.CSV(
					[]string{"ID", "Name", "Default Status", "Description", "Enabled Standard Fields"},
					[][]string{{
						fmt.Sprintf("%d", tracker.ID),
						tracker.Name,
						trackerDefaultStatusDetail(tracker),
						tracker.Description,
						trackerStandardFields(tracker),
					}},
				)
				return nil
			}

			details := []output.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", tracker.ID)},
				{Key: "Name", Value: tracker.Name},
			}
			if value := trackerDefaultStatusDetail(tracker); value != "" {
				details = append(details, output.KeyValue{Key: "Default Status", Value: value})
			}
			if tracker.Description != "" {
				details = append(details, output.KeyValue{Key: "Description", Value: tracker.Description})
			}
			if value := trackerStandardFields(tracker); value != "" {
				details = append(details, output.KeyValue{Key: "Enabled Standard Fields", Value: value})
			}

			printer.Detail(details)
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func trackerDefaultStatus(t models.Tracker) string {
	if t.DefaultStatus == nil {
		return ""
	}
	return t.DefaultStatus.Name
}

func trackerDefaultStatusDetail(t *models.Tracker) string {
	if t.DefaultStatus == nil {
		return ""
	}
	return fmt.Sprintf("%s (ID: %d)", t.DefaultStatus.Name, t.DefaultStatus.ID)
}

func trackerStandardFields(t *models.Tracker) string {
	if len(t.EnabledStandardFields) == 0 {
		return ""
	}
	return strings.Join(t.EnabledStandardFields, ", ")
}
