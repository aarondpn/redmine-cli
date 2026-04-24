package time

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdTimeList(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		user     string
		issue    int
		from     string
		to       string
		activity string
		limit    int
		offset   int
		format   string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List time entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()

			from = cmdutil.ResolveDateKeyword(from)
			to = cmdutil.ResolveDateKeyword(to)

			project, err = cmdutil.DefaultProjectID(ctx, f, project)
			if err != nil {
				return err
			}

			// Resolve user: if non-numeric and not "me", resolve by name
			userID := user
			if user != "" && user != "me" {
				if _, err := strconv.Atoi(user); err != nil {
					resolved, err := resolver.ResolveUser(ctx, client, user)
					if err != nil {
						return fmt.Errorf("resolving user: %w", err)
					}
					userID = strconv.Itoa(resolved)
				}
			}

			var activityID int
			if activity != "" {
				activityID, err = resolver.ResolveActivity(ctx, client, activity)
				if err != nil {
					return fmt.Errorf("resolving activity: %w", err)
				}
			}

			filter := models.TimeEntryFilter{
				ProjectID:  project,
				UserID:     userID,
				IssueID:    issue,
				From:       from,
				To:         to,
				ActivityID: activityID,
				Limit:      limit,
				Offset:     offset,
			}

			entries, total, err := client.TimeEntries.List(ctx, filter)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			headers := []string{"ID", "Date", "Project", "Issue", "Hours", "Activity", "User", "Comments"}

			if len(entries) == 0 && printer.Format() == output.FormatCSV {
				printer.CSV(headers, nil)
				return nil
			}

			if cmdutil.HandleEmpty(printer, entries, "time entries") {
				return nil
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(entries)
				return nil
			case output.FormatCSV:
				rows := make([][]string, len(entries))
				for i, e := range entries {
					issueRef := ""
					if e.Issue != nil {
						issueRef = strconv.Itoa(e.Issue.ID)
					}
					rows[i] = []string{
						strconv.Itoa(e.ID),
						e.SpentOn,
						e.Project.Name,
						issueRef,
						fmt.Sprintf("%.2f", e.Hours),
						e.Activity.Name,
						e.User.Name,
						e.Comments,
					}
				}
				printer.CSV(headers, rows)
				return nil
			}

			rows := make([][]string, len(entries))
			for i, e := range entries {
				issueRef := ""
				if e.Issue != nil {
					issueRef = fmt.Sprintf("#%d", e.Issue.ID)
				}
				rows[i] = []string{
					output.StyleID.Render(strconv.Itoa(e.ID)),
					e.SpentOn,
					e.Project.Name,
					issueRef,
					fmt.Sprintf("%.2f", e.Hours),
					e.Activity.Name,
					e.User.Name,
					e.Comments,
				}
			}

			printer.Table(headers, rows)
			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(entries), Total: total, Limit: limit, Offset: offset, Noun: "entries",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Filter by project name, identifier, or ID")
	cmd.Flags().StringVar(&user, "user", "", "Filter by user ID, login, name, or 'me'")
	cmd.Flags().IntVar(&issue, "issue", 0, "Filter by issue ID")
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD or 'today')")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD or 'today')")
	cmd.Flags().StringVar(&activity, "activity", "", "Filter by activity name or ID")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("activity", cmdutil.CompleteActivities(f))
	_ = cmd.RegisterFlagCompletionFunc("user", cmdutil.CompleteUsers(f))

	return cmd
}
