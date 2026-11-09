package issue

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
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
		attachDesc     []string
		customFieldRaw []string
		project        string
		removeAttach   []int
		force          bool
	)

	cmd := &cobra.Command{
		Use:     "update <id>",
		Aliases: []string{"edit"},
		Short:   "Update an issue",
		Long: "Update fields on an existing issue.\n\n" +
			"When --status is given, the issue is read back after the update. Redmine " +
			"silently ignores status transitions its workflow does not allow for your " +
			"role and the issue's tracker (and refuses to close issues with open subtasks " +
			"or open blocking issues), so if the status did not change the command fails " +
			"with error code status_not_applied, listing the allowed statuses (Redmine " +
			"5.0+). Other changes in the same request may already have been applied.\n\n" +
			"--project moves the issue and is verified the same way: Redmine silently " +
			"ignores a move to a project in which you may not add issues, so the command " +
			"then fails with error code project_not_applied. Category, version and " +
			"assignee names are resolved in the target project.\n\n" +
			"--remove-attachment deletes attachments of the issue and asks for " +
			"confirmation unless --force is given.",
		Example: `  # Update status and priority by name
  redmine issues update 123 --status Closed --priority Low

  # Reassign to yourself with a note
  redmine issues update 123 --assignee me --note "Taking over this issue"

  # Assign to a group (or user) by name, or clear the assignee
  redmine issues update 123 --assignee "Backend Team"
  redmine issues update 123 --assignee none

  # Clear a custom field (also clears every value of a multi-value field)
  redmine issues update 123 --custom-field Severity=

  # Add a private note (only visible to staff)
  redmine issues update 123 --note "Internal context" --private-notes

  # Read a long description or note from a file, or from stdin with "-"
  redmine issues update 123 --description-file description.textile
  generate-report | redmine issues update 123 --note-file -

  # Change category
  redmine issues update 123 --category "Development"

  # Set version, dates, and estimated hours
  redmine issues update 123 --version "v2.0" --start-date 2026-05-01 --due-date 2026-05-15 --estimated-hours 4.5

  # Update a custom field (by name or numeric ID)
  redmine issues update 123 --custom-field Severity=Critical

  # Move the issue to another project (name, identifier or ID)
  redmine issues update 123 --project other-project

  # Remove two attachments without asking
  redmine issues update 123 --remove-attachment 45 --remove-attachment 46 --force

  # Attach files with descriptions (paired by position)
  redmine issues update 123 --attach ./log.txt --attach-description "Server log"

  # Numeric IDs still work
  redmine issues update 123 --tracker 1 --status 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.CheckAttachDescriptions(attach, attachDesc); err != nil {
				return err
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}
			for _, aid := range removeAttach {
				if aid <= 0 {
					return fmt.Errorf("invalid attachment ID %d: --remove-attachment takes positive attachment IDs", aid)
				}
			}

			if err := cmdutil.RejectMultipleStdinFlags(cmd, "description-file", "note-file"); err != nil {
				return err
			}
			printer := f.Printer("")
			if len(removeAttach) > 0 && !force {
				// The prompt reads stdin, which a "-" file flag consumes.
				for _, name := range []string{"description-file", "note-file"} {
					if v, _ := cmd.Flags().GetString(name); v == "-" {
						return fmt.Errorf("--%s - reads standard input, so --remove-attachment cannot ask for confirmation: add --force", name)
					}
				}
				ids := make([]string, len(removeAttach))
				for i, aid := range removeAttach {
					ids[i] = strconv.Itoa(aid)
				}
				msg := fmt.Sprintf("Remove attachment(s) %s from issue #%d? This cannot be undone.", strings.Join(ids, ", "), id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionUpdated, "issue", id, "Update cancelled")
					return nil
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			update := ops.UpdateIssueInput{ID: id, DeletedAttachmentIDs: removeAttach}
			descriptionSet, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "description", "description-file", &description)
			if err != nil {
				return err
			}
			noteSet, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "note", "note-file", &note)
			if err != nil {
				return err
			}
			// Empty files are rejected: an empty note would be a silent
			// no-op and an empty description would silently wipe the
			// existing one. --description "" still clears it explicitly.
			if err := cmdutil.RejectEmptyTextFile(cmd, "description-file", description, cmdutil.ClearDescriptionHint); err != nil {
				return err
			}
			if err := cmdutil.RejectEmptyTextFile(cmd, "note-file", note, ""); err != nil {
				return err
			}
			if cmd.Flags().Changed("assignee") {
				if err := checkAssigneeFlag(assignee); err != nil {
					return err
				}
			}

			if cmd.Flags().Changed("subject") {
				update.Subject = &subject
			}
			if descriptionSet {
				update.Description = &description
			}
			if cmd.Flags().Changed("done-ratio") {
				update.DoneRatio = &doneRatio
			}
			if noteSet {
				update.Notes = &note
			}
			if cmd.Flags().Changed("private-notes") {
				if !noteSet {
					return fmt.Errorf("--private-notes requires --note or --note-file")
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

			// The target project of a move is also where names are resolved.
			var targetIdentifier string
			if cmd.Flags().Changed("project") {
				pid, identifier, err := resolver.ResolveProject(ctx, client, project)
				if err != nil {
					return fmt.Errorf("resolving project: %w", err)
				}
				update.ProjectID = &pid
				targetIdentifier = identifier
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
			// Assignee names are matched against the issue's project
			// memberships, so a name (not an ID, "me" or "none") also needs
			// the project.
			needsProject := (cmd.Flags().Changed("category") && func() bool { _, err := strconv.Atoi(category); return err != nil }()) ||
				(cmd.Flags().Changed("version") && func() bool { _, err := strconv.Atoi(version); return err != nil }()) ||
				(cmd.Flags().Changed("assignee") && resolver.NeedsPrincipalLookup(assignee))

			projectIdentifier := targetIdentifier
			if needsProject && projectIdentifier == "" {
				issue, err := ops.GetIssue(ctx, client, ops.GetIssueInput{ID: id})
				if err != nil {
					return fmt.Errorf("failed to fetch issue for name resolution: %w", err)
				}
				// Redmine accepts a numeric project ID anywhere the path expects
				// an identifier, so the issue's project ID is enough to feed the
				// downstream category/version list endpoints — no separate fetch.
				projectIdentifier = strconv.Itoa(issue.Project.ID)
			}

			if cmd.Flags().Changed("assignee") {
				// "none" resolves to 0, which clears the assignee.
				aid, err := resolveAssigneeValue(resolver.NewPrincipalResolver(ctx, client, projectIdentifier), assignee)
				if err != nil {
					return fmt.Errorf("resolving assignee: %w", err)
				}
				update.AssignedToID = &aid
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
				values, err := cmdutil.ParseCustomFieldValues(ctx, client, customFieldRaw, models.CustomizedTypeIssue)
				if err != nil {
					return err
				}
				update.CustomFieldValues = values
			}

			if len(attach) > 0 {
				uploads, err := cmdutil.UploadAttachments(ctx, client, attach, attachDesc...)
				if err != nil {
					return err
				}
				update.Uploads = uploads
			}

			stop := printer.Spinner("Updating issue...")
			result, err := ops.UpdateIssue(ctx, client, update)
			stop()
			if err != nil {
				if coded := statusNotAppliedError(err); coded != nil {
					return coded
				}
				if coded := projectNotAppliedError(err); coded != nil {
					return coded
				}
				return fmt.Errorf("failed to update issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printWarnings(printer, result.Warnings)
			printer.Action(output.ActionUpdated, "issue", id, fmt.Sprintf("Updated issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&subject, "subject", "", "Issue subject")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	cmdutil.AddTextFileFlag(cmd, "description", "description-file", "")
	cmd.Flags().StringVar(&tracker, "tracker", "", "Tracker name or ID")
	cmd.Flags().StringVar(&status, "status", "", "Status name or ID")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name or ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee: user or group name, login, ID, 'me', or 'none' to unassign")
	cmd.Flags().StringVar(&category, "category", "", "Issue category name or ID")
	cmd.Flags().StringVar(&version, "version", "", "Target version name or ID")
	cmd.Flags().IntVar(&parent, "parent", 0, "Parent issue ID (0 clears the parent)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD; empty clears)")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "Due date (YYYY-MM-DD; empty clears)")
	cmd.Flags().Float64Var(&estimatedHours, "estimated-hours", 0, "Estimated hours")
	cmd.Flags().BoolVar(&private, "private", false, "Mark issue as private")
	cmd.Flags().IntVar(&doneRatio, "done-ratio", 0, "Done ratio (0-100)")
	cmd.Flags().StringVar(&note, "note", "", "Add a note to the issue")
	cmdutil.AddTextFileFlag(cmd, "note", "note-file", "")
	cmd.Flags().BoolVar(&privateNotes, "private-notes", false, "Mark the note attached via --note or --note-file as private")
	cmdutil.AddAttachFlags(cmd, &attach, &attachDesc)
	cmdutil.AddCustomFieldFlag(cmd, f, &customFieldRaw, "Custom field", models.CustomizedTypeIssue)
	cmd.Flags().StringVar(&project, "project", "", "Move the issue to this project (name, identifier, or ID)")
	cmd.Flags().IntSliceVar(&removeAttach, "remove-attachment", nil, "Attachment ID to remove from the issue (repeatable; asks for confirmation)")
	cmdutil.AddForceFlag(cmd, &force)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteStatuses(f))
	_ = cmd.RegisterFlagCompletionFunc("priority", cmdutil.CompletePriorities(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteAssignees(f))
	_ = cmd.RegisterFlagCompletionFunc("category", cmdutil.CompleteCategories(f))
	_ = cmd.RegisterFlagCompletionFunc("version", cmdutil.CompleteOpenVersions(f))

	return cmd
}
