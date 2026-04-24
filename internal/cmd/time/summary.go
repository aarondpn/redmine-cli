package time

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdTimeSummary(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		user    string
		from    string
		to      string
		groupBy string
		format  string
	)

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarize time entries",
		Long:  "Aggregate time entries by day, project, or activity.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.DefaultProjectID(context.Background(), f, project)
			if err != nil {
				return err
			}

			from = cmdutil.ResolveDateKeyword(from)
			to = cmdutil.ResolveDateKeyword(to)

			if from == "" {
				// Default to start of current week (Monday)
				now := time.Now()
				weekday := int(now.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				monday := now.AddDate(0, 0, -(weekday - 1))
				from = monday.Format("2006-01-02")
			}
			if to == "" {
				to = time.Now().Format("2006-01-02")
			}

			// Resolve user: if non-numeric and not "me", resolve by name
			userID := user
			if user != "" && user != "me" {
				if _, err := strconv.Atoi(user); err != nil {
					resolved, err := resolver.ResolveUser(context.Background(), client, user)
					if err != nil {
						return fmt.Errorf("resolving user: %w", err)
					}
					userID = strconv.Itoa(resolved)
				}
			}

			// Fetch all entries in the range (use a large limit)
			filter := models.TimeEntryFilter{
				ProjectID: project,
				UserID:    userID,
				From:      from,
				To:        to,
				Limit:     0, // fetch all
			}

			entries, _, err := client.TimeEntries.List(context.Background(), filter)
			if err != nil {
				return err
			}

			// Aggregate by group
			totals := make(map[string]float64)
			for _, e := range entries {
				var key string
				switch groupBy {
				case "project":
					key = e.Project.Name
				case "activity":
					key = e.Activity.Name
				default: // "day"
					key = e.SpentOn
				}
				totals[key] += e.Hours
			}

			// Sort keys
			keys := make([]string, 0, len(totals))
			for k := range totals {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			printer := f.Printer(format)

			switch printer.Format() {
			case output.FormatJSON:
				type summaryRow struct {
					Group string  `json:"group"`
					Hours float64 `json:"hours"`
				}
				rows := make([]summaryRow, len(keys))
				for i, k := range keys {
					rows[i] = summaryRow{Group: k, Hours: totals[k]}
				}
				printer.JSON(rows)
				return nil
			case output.FormatCSV:
				headers := []string{"Group", "Total Hours"}
				rows := make([][]string, len(keys))
				for i, k := range keys {
					rows[i] = []string{k, fmt.Sprintf("%.2f", totals[k])}
				}
				printer.CSV(headers, rows)
				return nil
			}

			headers := []string{"Group", "Total Hours"}
			rows := make([][]string, len(keys))
			var grandTotal float64
			for i, k := range keys {
				rows[i] = []string{k, fmt.Sprintf("%.2f", totals[k])}
				grandTotal += totals[k]
			}

			printer.Table(headers, rows)
			printer.Success(fmt.Sprintf("Total: %.2f hours (%s to %s)", grandTotal, from, to))

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Filter by project name, identifier, or ID")
	cmd.Flags().StringVar(&user, "user", "", "Filter by user ID, login, name, or 'me'")
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD or 'today', default: start of current week)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD or 'today', default: today)")
	cmd.Flags().StringVar(&groupBy, "group-by", "day", "Group by: day, project, activity")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("user", cmdutil.CompleteUsers(f))

	return cmd
}
