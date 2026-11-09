package issue

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdCopy creates the issues copy command.
func NewCmdCopy(f *cmdutil.Factory) *cobra.Command {
	var (
		project         string
		subject         string
		tracker         string
		status          string
		priority        string
		assignee        string
		link            bool
		withAttachments bool
		withSubtasks    bool
		format          string
	)

	cmd := &cobra.Command{
		Use:   "copy <id>",
		Short: "Copy an issue",
		Long: "Create a copy of an existing issue, in the same or another project.\n\n" +
			"Redmine copies the issue's fields, custom fields and parent; the copy starts " +
			"in the default status and is authored by you. Watchers are copied when you " +
			"may add watchers in the target project. The flags override the copied " +
			"values. Redmine's \"Link issues on copy\" setting (and, on Redmine 6.0+, " +
			"\"Copy attachments on copy\") can force or forbid --link and " +
			"--with-attachments regardless. Requires the Copy issues permission.",
		Example: `  # Copy an issue within its project
  redmine issues copy 123

  # Copy into another project, linked to the original, with attachments and subtasks
  redmine issues copy 123 --project other-project --link --with-attachments --with-subtasks

  # Copy with a new subject and assignee
  redmine issues copy 123 --subject "Same bug on staging" --assignee me`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}
			if err := checkAssigneeFlag(assignee); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			ctx := cmdutil.Context(cmd)

			input := ops.CopyIssueInput{
				ID:              id,
				Subject:         subject,
				LinkCopy:        link,
				CopyAttachments: withAttachments,
				CopySubtasks:    withSubtasks,
			}

			// Assignee names are matched against the target project's
			// memberships.
			var projectIdentifier string
			if project != "" {
				pid, identifier, err := resolver.ResolveProject(ctx, client, project)
				if err != nil {
					return fmt.Errorf("resolving project: %w", err)
				}
				input.ProjectID = pid
				projectIdentifier = identifier
			} else if assignee != "" && resolver.NeedsPrincipalLookup(assignee) {
				source, err := ops.GetIssue(ctx, client, ops.GetIssueInput{ID: id})
				if err != nil {
					return fmt.Errorf("failed to fetch issue #%d: %w", id, err)
				}
				input.ProjectID = source.Project.ID
				projectIdentifier = strconv.Itoa(source.Project.ID)
			}

			if tracker != "" {
				tid, err := resolver.ResolveTracker(ctx, client, tracker)
				if err != nil {
					return fmt.Errorf("resolving tracker: %w", err)
				}
				input.TrackerID = tid
			}
			if status != "" {
				sid, err := resolver.ResolveStatus(ctx, client, status)
				if err != nil {
					return fmt.Errorf("resolving status: %w", err)
				}
				input.StatusID = sid
			}
			if priority != "" {
				pid, err := resolver.ResolvePriority(ctx, client, priority)
				if err != nil {
					return fmt.Errorf("resolving priority: %w", err)
				}
				input.PriorityID = pid
			}
			if assignee != "" {
				// "none" resolves to 0, which leaves the copy unassigned.
				aid, err := resolveAssigneeValue(resolver.NewPrincipalResolver(ctx, client, projectIdentifier), assignee)
				if err != nil {
					return fmt.Errorf("resolving assignee: %w", err)
				}
				input.AssignedToID = &aid
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Copying issue...")
			issue, err := ops.CopyIssue(ctx, client, input)
			stop()
			if err != nil {
				return fmt.Errorf("failed to copy issue #%d: %w", id, err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(issue)
				return nil
			}

			printer.Success(fmt.Sprintf("Copied issue #%d to #%d: %s", id, issue.ID, issue.Subject))
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

	cmd.Flags().StringVar(&project, "project", "", "Project to create the copy in (name, identifier, or ID; default: the source issue's project)")
	cmd.Flags().StringVar(&subject, "subject", "", "Subject of the copy (default: the source subject)")
	cmd.Flags().StringVar(&tracker, "tracker", "", "Tracker name or ID")
	cmd.Flags().StringVar(&status, "status", "", "Status name or ID (default: the tracker's default status)")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name or ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee: user or group name, login, ID, 'me', or 'none' to leave the copy unassigned")
	cmd.Flags().BoolVar(&link, "link", false, "Relate the copy to the original (copied to / copied from)")
	cmd.Flags().BoolVar(&withAttachments, "with-attachments", false, "Copy the attachments")
	cmd.Flags().BoolVar(&withSubtasks, "with-subtasks", false, "Copy the subtasks")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteStatuses(f))
	_ = cmd.RegisterFlagCompletionFunc("priority", cmdutil.CompletePriorities(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteAssignees(f))

	return cmd
}
