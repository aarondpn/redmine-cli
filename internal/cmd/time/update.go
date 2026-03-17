package time

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/resolver"
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

			update := models.TimeEntryUpdate{}

			if cmd.Flags().Changed("hours") {
				update.Hours = &hours
			}
			if cmd.Flags().Changed("activity") {
				activityID, err := resolver.ResolveActivity(context.Background(), client, activity)
				if err != nil {
					return err
				}
				update.ActivityID = &activityID
			}
			if cmd.Flags().Changed("date") {
				update.SpentOn = &date
			}
			if cmd.Flags().Changed("comment") {
				update.Comments = &comment
			}

			if err := client.TimeEntries.Update(context.Background(), id, update); err != nil {
				return err
			}

			printer := f.Printer("")
			printer.Success(fmt.Sprintf("Time entry #%d updated", id))

			return nil
		},
	}

	cmd.Flags().Float64Var(&hours, "hours", 0, "Hours spent")
	cmd.Flags().StringVar(&activity, "activity", "", "Activity name or ID")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment")

	return cmd
}
