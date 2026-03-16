package issue

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdCreate creates the issues create command.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project     int
		tracker     int
		subject     string
		description string
		priority    int
		assignee    int
		format      string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create a new issue",
		Long:    "Create a new issue in the specified project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			create := models.IssueCreate{
				ProjectID:    project,
				TrackerID:    tracker,
				Subject:      subject,
				Description:  description,
				PriorityID:   priority,
				AssignedToID: assignee,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Creating issue...")
			issue, err := client.Issues.Create(context.Background(), create)
			stop()
			if err != nil {
				return fmt.Errorf("failed to create issue: %w", err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(issue)
				return nil
			}

			printer.Success(fmt.Sprintf("Created issue %s: %s", fmt.Sprintf("#%d", issue.ID), issue.Subject))

			printer.Detail([]output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(fmt.Sprintf("#%d", issue.ID))},
				{Key: "Project", Value: issue.Project.Name},
				{Key: "Tracker", Value: issue.Tracker.Name},
				{Key: "Status", Value: issue.Status.Name},
				{Key: "Priority", Value: issue.Priority.Name},
				{Key: "Subject", Value: issue.Subject},
				{Key: "Assignee", Value: assigneeName(issue.AssignedTo)},
			})

			return nil
		},
	}

	cmd.Flags().IntVar(&project, "project", 0, "Project ID (required)")
	cmd.Flags().IntVar(&tracker, "tracker", 0, "Tracker ID")
	cmd.Flags().StringVar(&subject, "subject", "", "Issue subject (required)")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	cmd.Flags().IntVar(&priority, "priority", 0, "Priority ID")
	cmd.Flags().IntVar(&assignee, "assignee", 0, "Assignee user ID")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("subject")

	return cmd
}
