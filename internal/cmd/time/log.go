package time

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// timeLogValidationHint explains the usual causes of a 422 when logging time.
const timeLogValidationHint = "The server may require an activity or time entry custom fields: pass --activity and --custom-field name=value " +
	"(definitions: `redmine custom-fields list`, admin only; values of an existing entry: `redmine time get <id> -o json`)."

func newCmdTimeLog(f *cmdutil.Factory) *cobra.Command {
	var (
		issue          int
		project        string
		hours          float64
		activity       string
		date           string
		comment        string
		user           string
		format         string
		customFieldRaw []string
	)

	cmd := &cobra.Command{
		Use:     "log",
		Args:    cobra.NoArgs,
		Aliases: []string{"add", "create"},
		Short:   "Log a time entry",
		Example: `  # Log time against an issue
  redmine time log --issue 123 --hours 1.5 --activity Development --comment "Bug fix"

  # Set time entry custom fields (by name or numeric ID); repeat a field for multi-value fields
  redmine time log --issue 123 --hours 2 --custom-field "Billing Code=ACME-42" \
    --custom-field Tags=Onsite --custom-field Tags=Overtime`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)

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
				// Names are matched against project members; with only --issue
				// the project is the issue's, fetched just for a name lookup. A
				// failed fetch only skips the membership lookup: the time entry
				// request reports a missing issue on its own.
				userProject := project
				if userProject == "" && issue > 0 && resolver.NeedsPrincipalLookup(user) {
					if iss, err := client.Issues.Get(ctx, issue, nil); err == nil {
						userProject = strconv.Itoa(iss.Project.ID)
					}
				}
				userID, err = resolver.ResolveUserInProject(ctx, client, user, userProject)
				if err != nil {
					return fmt.Errorf("resolving user: %w", err)
				}
			}

			input := ops.CreateTimeEntryInput{
				IssueID:    issue,
				ProjectID:  project,
				Hours:      hours,
				ActivityID: activityID,
				SpentOn:    date,
				Comments:   comment,
				UserID:     userID,
			}
			if len(customFieldRaw) > 0 {
				values, err := cmdutil.ParseCustomFieldValues(ctx, client, customFieldRaw, models.CustomizedTypeTimeEntry)
				if err != nil {
					return err
				}
				input.CustomFieldValues = values
			}

			created, err := ops.CreateTimeEntry(ctx, client, input)
			if err != nil {
				return cmdutil.WithValidationHint(err, timeLogValidationHint, cmdutil.CustomFieldValidationHint(customFieldRaw))
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
	cmdutil.AddCustomFieldFlag(cmd, f, &customFieldRaw, "Time entry custom field", models.CustomizedTypeTimeEntry)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("hours")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("activity", cmdutil.CompleteActivities(f))
	_ = cmd.RegisterFlagCompletionFunc("user", cmdutil.CompleteUsers(f))

	return cmd
}
