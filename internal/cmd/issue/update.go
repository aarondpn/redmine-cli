package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdUpdate creates the issues update command.
func NewCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		subject        string
		description    string
		tracker        string
		status         string
		priority       string
		assignee       string
		category       string
		version        string
		parent         int
		startDate      string
		dueDate        string
		estimatedHours float64
		private        bool
		doneRatio      int
		note           string
		privateNotes   bool
		attach         []string
		customFieldRaw []string
	)

	cmd := &cobra.Command{
		Use:     "update <id>",
		Aliases: []string{"edit"},
		Short:   "Update an issue",
		Long:    "Update fields on an existing issue.",
		Example: `  # Update status and priority by name
  redmine issues update 123 --status Closed --priority Low

  # Reassign to yourself with a note
  redmine issues update 123 --assignee me --note "Taking over this issue"

  # Add a private note (only visible to staff)
  redmine issues update 123 --note "Internal context" --private-notes

  # Change category
  redmine issues update 123 --category "Development"

  # Set version, dates, and estimated hours
  redmine issues update 123 --version "v2.0" --start-date 2026-05-01 --due-date 2026-05-15 --estimated-hours 4.5

  # Update a custom field (by name or numeric ID)
  redmine issues update 123 --custom-field Severity=Critical

  # Numeric IDs still work
  redmine issues update 123 --tracker 1 --status 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			update := ops.UpdateIssueInput{ID: id}

			if cmd.Flags().Changed("subject") {
				update.Subject = &subject
			}
			if cmd.Flags().Changed("description") {
				update.Description = &description
			}
			if cmd.Flags().Changed("done-ratio") {
				update.DoneRatio = &doneRatio
			}
			if cmd.Flags().Changed("note") {
				update.Notes = &note
			}
			if cmd.Flags().Changed("private-notes") {
				if !cmd.Flags().Changed("note") {
					return fmt.Errorf("--private-notes requires --note")
				}
				update.PrivateNotes = &privateNotes
			}
			if cmd.Flags().Changed("parent") {
				update.ParentIssueID = &parent
			}
			if cmd.Flags().Changed("start-date") {
				update.StartDate = &startDate
			}
			if cmd.Flags().Changed("due-date") {
				update.DueDate = &dueDate
			}
			if cmd.Flags().Changed("estimated-hours") {
				update.EstimatedHours = &estimatedHours
			}
			if cmd.Flags().Changed("private") {
				update.IsPrivate = &private
			}

			if cmd.Flags().Changed("tracker") {
				tid, err := resolver.ResolveTracker(ctx, client, tracker)
				if err != nil {
					return fmt.Errorf("resolving tracker: %w", err)
				}
				update.TrackerID = &tid
			}
			if cmd.Flags().Changed("status") {
				sid, err := resolver.ResolveStatus(ctx, client, status)
				if err != nil {
					return fmt.Errorf("resolving status: %w", err)
				}
				update.StatusID = &sid
			}
			if cmd.Flags().Changed("priority") {
				pid, err := resolver.ResolvePriority(ctx, client, priority)
				if err != nil {
					return fmt.Errorf("resolving priority: %w", err)
				}
				update.PriorityID = &pid
			}
			if cmd.Flags().Changed("assignee") {
				aid, err := resolver.ResolveAssignee(ctx, client, assignee)
				if err != nil {
					return fmt.Errorf("resolving assignee: %w", err)
				}
				update.AssignedToID = &aid
			}
			needsProject := (cmd.Flags().Changed("category") && func() bool { _, err := strconv.Atoi(category); return err != nil }()) ||
				(cmd.Flags().Changed("version") && func() bool { _, err := strconv.Atoi(version); return err != nil }())

			var projectIdentifier string
			if needsProject {
				issue, err := ops.GetIssue(ctx, client, ops.GetIssueInput{ID: id})
				if err != nil {
					return fmt.Errorf("failed to fetch issue for name resolution: %w", err)
				}
				// Redmine accepts a numeric project ID anywhere the path expects
				// an identifier, so the issue's project ID is enough to feed the
				// downstream category/version list endpoints — no separate fetch.
				projectIdentifier = strconv.Itoa(issue.Project.ID)
			}

			if cmd.Flags().Changed("category") {
				cid, err := resolver.ResolveCategory(ctx, client, category, projectIdentifier)
				if err != nil {
					return fmt.Errorf("resolving category: %w", err)
				}
				update.CategoryID = &cid
			}
			if cmd.Flags().Changed("version") {
				vid, err := resolver.ResolveVersion(ctx, client, version, projectIdentifier)
				if err != nil {
					return fmt.Errorf("resolving version: %w", err)
				}
				update.FixedVersionID = &vid
			}

			if len(customFieldRaw) > 0 {
				values, err := cmdutil.ParseCustomFieldValues(ctx, client, customFieldRaw)
				if err != nil {
					return err
				}
				update.CustomFieldValues = values
			}

			if len(attach) > 0 {
				uploads, err := cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
				update.Uploads = uploads
			}

			printer := f.Printer("")
			stop := printer.Spinner("Updating issue...")
			_, err = ops.UpdateIssue(ctx, client, update)
			stop()
			if err != nil {
				return fmt.Errorf("failed to update issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Action(output.ActionUpdated, "issue", id, fmt.Sprintf("Updated issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&subject, "subject", "", "Issue subject")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	cmd.Flags().StringVar(&tracker, "tracker", "", "Tracker name or ID")
	cmd.Flags().StringVar(&status, "status", "", "Status name or ID")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name or ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee name, login, ID, or 'me'")
	cmd.Flags().StringVar(&category, "category", "", "Issue category name or ID")
	cmd.Flags().StringVar(&version, "version", "", "Target version name or ID")
	cmd.Flags().IntVar(&parent, "parent", 0, "Parent issue ID (0 clears the parent)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD; empty clears)")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "Due date (YYYY-MM-DD; empty clears)")
	cmd.Flags().Float64Var(&estimatedHours, "estimated-hours", 0, "Estimated hours")
	cmd.Flags().BoolVar(&private, "private", false, "Mark issue as private")
	cmd.Flags().IntVar(&doneRatio, "done-ratio", 0, "Done ratio (0-100)")
	cmd.Flags().StringVar(&note, "note", "", "Add a note to the issue")
	cmd.Flags().BoolVar(&privateNotes, "private-notes", false, "Mark the note attached via --note as private (requires --note)")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")
	cmd.Flags().StringArrayVar(&customFieldRaw, "custom-field", nil, "Custom field value as name=value or id=value (repeatable)")

	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteStatuses(f))
	_ = cmd.RegisterFlagCompletionFunc("priority", cmdutil.CompletePriorities(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteUsers(f))
	_ = cmd.RegisterFlagCompletionFunc("category", cmdutil.CompleteCategories(f))
	_ = cmd.RegisterFlagCompletionFunc("version", cmdutil.CompleteOpenVersions(f))

	return cmd
}
