package issue

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdCreate creates the issues create command.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project        string
		tracker        string
		subject        string
		description    string
		priority       string
		assignee       string
		status         string
		category       string
		version        string
		parent         int
		estimatedHours float64
		private        bool
		attach         []string
		format         string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create a new issue",
		Long:    "Create a new issue in the specified project.",
		Example: `  # Create an issue using names instead of IDs
  redmine issues create --project myproject --tracker Bug --priority High --subject "Fix login"

  # Assign to yourself
  redmine issues create --project myproject --subject "My task" --assignee me

  # Create with all fields
  redmine issues create --project myproject --tracker Feature --priority Normal \
    --subject "Add search" --description "Full-text search" \
    --assignee "John Smith" --category "Development" --version "v2.0" --estimated-hours 8 --private

  # Numeric IDs still work
  redmine issues create --project 1 --tracker 1 --priority 2 --subject "Test"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()

			project = cmdutil.DefaultProject(f, project)
			if project == "" {
				return fmt.Errorf("--project is required (or set a default project in config)")
			}

			projectID, projectIdentifier, err := resolver.ResolveProject(ctx, client, project)
			if err != nil {
				return fmt.Errorf("resolving project: %w", err)
			}

			create := models.IssueCreate{
				ProjectID:      projectID,
				Subject:        subject,
				Description:    description,
				ParentIssueID:  parent,
				EstimatedHours: estimatedHours,
			}

			if tracker != "" {
				id, err := resolver.ResolveTracker(ctx, client, tracker)
				if err != nil {
					return fmt.Errorf("resolving tracker: %w", err)
				}
				create.TrackerID = id
			}

			if priority != "" {
				id, err := resolver.ResolvePriority(ctx, client, priority)
				if err != nil {
					return fmt.Errorf("resolving priority: %w", err)
				}
				create.PriorityID = id
			}

			if assignee != "" {
				id, err := resolver.ResolveAssignee(ctx, client, assignee)
				if err != nil {
					return fmt.Errorf("resolving assignee: %w", err)
				}
				create.AssignedToID = id
			}

			if status != "" {
				id, err := resolver.ResolveStatus(ctx, client, status)
				if err != nil {
					return fmt.Errorf("resolving status: %w", err)
				}
				create.StatusID = id
			}

			if category != "" {
				id, err := resolver.ResolveCategory(ctx, client, category, projectIdentifier)
				if err != nil {
					return fmt.Errorf("resolving category: %w", err)
				}
				create.CategoryID = id
			}

			if version != "" {
				id, err := resolver.ResolveVersion(ctx, client, version, projectIdentifier)
				if err != nil {
					return fmt.Errorf("resolving version: %w", err)
				}
				create.FixedVersionID = id
			}

			if cmd.Flags().Changed("private") {
				create.IsPrivate = &private
			}

			if len(attach) > 0 {
				uploads, err := cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
				create.Uploads = uploads
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Creating issue...")
			issue, err := client.Issues.Create(ctx, create)
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

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID")
	cmd.Flags().StringVar(&tracker, "tracker", "", "Tracker name or ID")
	cmd.Flags().StringVar(&subject, "subject", "", "Issue subject (required)")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name or ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee name, login, ID, or 'me'")
	cmd.Flags().StringVar(&status, "status", "", "Status name or ID")
	cmd.Flags().StringVar(&category, "category", "", "Issue category name or ID")
	cmd.Flags().StringVar(&version, "version", "", "Target version name or ID")
	cmd.Flags().IntVar(&parent, "parent", 0, "Parent issue ID")
	cmd.Flags().Float64Var(&estimatedHours, "estimated-hours", 0, "Estimated hours")
	cmd.Flags().BoolVar(&private, "private", false, "Mark issue as private")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("subject")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteStatuses(f))
	_ = cmd.RegisterFlagCompletionFunc("priority", cmdutil.CompletePriorities(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteUsers(f))
	_ = cmd.RegisterFlagCompletionFunc("category", cmdutil.CompleteCategories(f))
	_ = cmd.RegisterFlagCompletionFunc("version", cmdutil.CompleteOpenVersions(f))

	return cmd
}
