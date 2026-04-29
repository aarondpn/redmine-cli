package time

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdTimeUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		hours    float64
		activity string
		date     string
		comment  string
	)

	cmd := &cobra.Command{
		Use:     "update <id>",
		Aliases: []string{"edit"},
		Short:   "Update a time entry",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid time entry ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			input := ops.UpdateTimeEntryInput{ID: id}

			if cmd.Flags().Changed("hours") {
				input.Hours = &hours
			}
			if cmd.Flags().Changed("activity") {
				activityID, err := resolver.ResolveActivity(context.Background(), client, activity)
				if err != nil {
					return fmt.Errorf("resolving activity: %w", err)
				}
				input.ActivityID = &activityID
			}
			if cmd.Flags().Changed("date") {
				resolved := cmdutil.ResolveDateKeyword(date)
				input.SpentOn = &resolved
			}
			if cmd.Flags().Changed("comment") {
				input.Comments = &comment
			}

			if _, err := ops.UpdateTimeEntry(context.Background(), client, input); err != nil {
				return err
			}

			printer := f.Printer("")
			printer.Action(output.ActionUpdated, "time_entry", id, fmt.Sprintf("Time entry #%d updated", id))

			return nil
		},
	}

	cmd.Flags().Float64Var(&hours, "hours", 0, "Hours spent")
	cmd.Flags().StringVar(&activity, "activity", "", "Activity name or ID")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD or 'today')")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment")

	_ = cmd.RegisterFlagCompletionFunc("activity", cmdutil.CompleteActivities(f))

	return cmd
}
