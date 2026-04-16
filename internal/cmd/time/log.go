package time

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/aarondpn/redmine-cli/internal/resolver"
)

func newCmdTimeLog(f *cmdutil.Factory) *cobra.Command {
	var (
		issue    int
		project  string
		hours    float64
		activity string
		date     string
		comment  string
	)

	cmd := &cobra.Command{
		Use:     "log",
		Aliases: []string{"add", "create"},
		Short:   "Log a time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.DefaultProjectID(context.Background(), f, project)
			if err != nil {
				return err
			}

			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			var activityID int
			if activity != "" {
				activityID, err = resolver.ResolveActivity(context.Background(), client, activity)
				if err != nil {
					return fmt.Errorf("resolving activity: %w", err)
				}
			}

			entry := models.TimeEntryCreate{
				IssueID:    issue,
				ProjectID:  project,
				Hours:      hours,
				ActivityID: activityID,
				SpentOn:    date,
				Comments:   comment,
			}

			created, err := client.TimeEntries.Create(context.Background(), entry)
			if err != nil {
				return err
			}

			printer := f.Printer("")
			printer.Action(output.ActionLogged, "time_entry", created.ID,
				fmt.Sprintf("Time entry #%s created (%.2f hours on %s)",
					strconv.Itoa(created.ID), created.Hours, created.SpentOn))

			return nil
		},
	}

	cmd.Flags().IntVar(&issue, "issue", 0, "Issue ID")
	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID")
	cmd.Flags().Float64Var(&hours, "hours", 0, "Hours spent (required)")
	cmd.Flags().StringVar(&activity, "activity", "", "Activity name or ID")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD, default today)")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment")

	_ = cmd.MarkFlagRequired("hours")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("activity", cmdutil.CompleteActivities(f))

	return cmd
}
