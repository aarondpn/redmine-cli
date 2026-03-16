package issue

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdGet creates the issues get command.
func NewCmdGet(f *cmdutil.Factory) *cobra.Command {
	var (
		include string
		format  string
	)

	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"show", "view"},
		Short:   "Get issue details",
		Long:    "Display detailed information about a specific issue.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			var includes []string
			if include != "" {
				includes = strings.Split(include, ",")
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching issue...")
			issue, err := client.Issues.Get(context.Background(), id, includes)
			stop()
			if err != nil {
				return fmt.Errorf("failed to get issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(issue)
				return nil
			}

			pairs := []output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(fmt.Sprintf("#%d", issue.ID))},
				{Key: "Project", Value: issue.Project.Name},
				{Key: "Tracker", Value: issue.Tracker.Name},
				{Key: "Status", Value: output.StatusStyle(issue.Status.Name).Render(issue.Status.Name)},
				{Key: "Priority", Value: output.PriorityStyle(issue.Priority.Name).Render(issue.Priority.Name)},
				{Key: "Subject", Value: issue.Subject},
				{Key: "Author", Value: issue.Author.Name},
				{Key: "Assignee", Value: assigneeName(issue.AssignedTo)},
				{Key: "Done Ratio", Value: fmt.Sprintf("%d%%", issue.DoneRatio)},
				{Key: "Created", Value: issue.CreatedOn},
				{Key: "Updated", Value: issue.UpdatedOn},
			}

			if issue.StartDate != "" {
				pairs = append(pairs, output.KeyValue{Key: "Start Date", Value: issue.StartDate})
			}
			if issue.DueDate != "" {
				pairs = append(pairs, output.KeyValue{Key: "Due Date", Value: issue.DueDate})
			}
			if issue.EstimatedHours != nil {
				pairs = append(pairs, output.KeyValue{Key: "Estimated Hours", Value: fmt.Sprintf("%.2f", *issue.EstimatedHours)})
			}
			if issue.FixedVersion != nil {
				pairs = append(pairs, output.KeyValue{Key: "Version", Value: issue.FixedVersion.Name})
			}
			if issue.Parent != nil {
				pairs = append(pairs, output.KeyValue{Key: "Parent", Value: fmt.Sprintf("#%d", issue.Parent.ID)})
			}
			if issue.Description != "" {
				pairs = append(pairs, output.KeyValue{Key: "Description", Value: issue.Description})
			}

			printer.Detail(pairs)

			if len(issue.Journals) > 0 {
				fmt.Println()
				for _, j := range issue.Journals {
					if j.Notes != "" {
						fmt.Printf("--- %s (%s) ---\n%s\n\n", j.User.Name, j.CreatedOn, j.Notes)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&include, "include", "", "Include related data: journals,children,relations")
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
