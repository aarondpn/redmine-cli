package time

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdTimeLog(f *cmdutil.Factory) *cobra.Command {
	var (
		issue    int
		project  string
		hours    float64
		activity string
		date     string
		comment  string
		user     string
		format   string
	)

	cmd := &cobra.Command{
		Use:     "log",
		Args:    cobra.NoArgs,
		Aliases: []string{"add", "create"},
		Short:   "Log a time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()

			// When logging against a specific issue, Redmine derives the project
			// from the issue. Applying the configured default project would inject
			// a conflicting project_id that Redmine can reject. Only fall back to
			// the default project when no issue is given; an explicit --project is
			// still honored.
			if issue > 0 {
				project, err = cmdutil.ResolveProjectID(ctx, f, project)
			} else {
				project, err = cmdutil.DefaultProjectID(ctx, f, project)
			}
			if err != nil {
				return err
			}

			date = cmdutil.ResolveDateKeyword(date)
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			var activityID int
			if activity != "" {
				activityID, err = resolver.ResolveActivity(ctx, client, activity)
				if err != nil {
					return fmt.Errorf("resolving activity: %w", err)
				}
			}

			var userID int
			if user != "" {
				userID, err = resolver.ResolveUser(ctx, client, user)
				if err != nil {
					return fmt.Errorf("resolving user: %w", err)
				}
			}

			created, err := ops.CreateTimeEntry(ctx, client, ops.CreateTimeEntryInput{
				IssueID:    issue,
				ProjectID:  project,
				Hours:      hours,
				ActivityID: activityID,
				SpentOn:    date,
				Comments:   comment,
				UserID:     userID,
			})
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			printer.Resource(created,
				fmt.Sprintf("Time entry #%s created (%.2f hours on %s)",
					strconv.Itoa(created.ID), created.Hours, created.SpentOn))

			return nil
		},
	}

	cmd.Flags().IntVar(&issue, "issue", 0, "Issue ID")
	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID")
	cmd.Flags().Float64Var(&hours, "hours", 0, "Hours spent (required)")
	cmd.Flags().StringVar(&activity, "activity", "", "Activity name or ID")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD or 'today', default today)")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment")
	cmd.Flags().StringVar(&user, "user", "", "Log time on behalf of a user (ID, login, name, or 'me'); requires admin or 'log time for other users' permission")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("hours")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("activity", cmdutil.CompleteActivities(f))
	_ = cmd.RegisterFlagCompletionFunc("user", cmdutil.CompleteUsers(f))

	return cmd
}
