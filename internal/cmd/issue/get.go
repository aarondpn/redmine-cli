package issue

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// NewCmdGet creates the issues get command.
func NewCmdGet(f *cmdutil.Factory) *cobra.Command {
	var (
		includes            []string
		journals            bool
		children            bool
		relations           bool
		attachments         bool
		downloadAttachments string
		clobber             bool
		allowHidden         bool
		format              string
	)

	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"show", "view"},
		Short:   "Get issue details",
		Long: "Display detailed information about a specific issue.\n\n" +
			"Use --attachments to list the issue's attachments (with their IDs, so you can\n" +
			"download one via `redmine attachments download <id>`), or --download-attachments\n" +
			"<dir> to download every attachment of the issue into <dir> in one step.\n\n" +
			"Existing files in <dir> are never overwritten unless --clobber is given, and\n" +
			"attachments whose filename starts with \".\" (such as .bashrc) are refused\n" +
			"unless --allow-hidden is given. Every name is checked before anything is\n" +
			"downloaded.\n\n" +
			"Attachments of the same issue that share a filename (compared ignoring case)\n" +
			"are all kept: the newest (highest attachment ID) keeps its name and older\n" +
			"ones are saved as \"<name> (attachment-<id>).<ext>\".",
		Example: `  # Show an issue
  redmine issues get 123

  # Include comments/history
  redmine issues get 123 --journals

  # List the issue's attachments (id, filename, size, type)
  redmine issues get 123 --attachments

  # Download every attachment into ./issue-123
  redmine issues get 123 --download-attachments ./issue-123

  # Re-download into the same directory, replacing earlier copies
  redmine issues get 123 --download-attachments ./issue-123 --clobber

  # Read a ticket fully before investigating: history, files, children, relations
  redmine issues get 123 --journals --children --relations --download-attachments ./issue-123
  redmine issues get 123 --include journals,attachments,children,relations -o json

  # Which statuses may the current user move the issue to? (Redmine 5.0+)
  redmine issues get 123 --include allowed_statuses -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)

			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			if err := validateGetIncludes(includes); err != nil {
				return err
			}
			var shorthands []string
			if journals {
				shorthands = append(shorthands, "journals")
			}
			if children {
				shorthands = append(shorthands, "children")
			}
			if relations {
				shorthands = append(shorthands, "relations")
			}
			if attachments || downloadAttachments != "" {
				shorthands = append(shorthands, "attachments")
			}
			includes = cmdutil.MergeIncludes(includes, shorthands...)

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching issue...")
			issue, err := ops.GetIssue(ctx, client, ops.GetIssueInput{ID: id, Includes: includes})
			stop()
			if err != nil {
				return fmt.Errorf("failed to get issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			// Side-effect: download every attachment into the target dir.
			// Confirmations go to stderr so JSON stdout stays the issue body.
			if downloadAttachments != "" {
				if err := downloadIssueAttachments(ctx, client, printer, issue, downloadAttachments, cmdutil.SaveOptions{Clobber: clobber, AllowHidden: allowHidden}); err != nil {
					return err
				}
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
						fmt.Printf("--- %s (%s) ---\n%s\n\n", output.SanitizeTerminal(j.User.Name), output.SanitizeTerminal(j.CreatedOn), output.SanitizeTerminal(j.Notes))
					}
				}
			}

			if len(issue.Attachments) > 0 {
				fmt.Println()
				fmt.Println("Attachments:")
				rows := make([][]string, len(issue.Attachments))
				for i, a := range issue.Attachments {
					rows[i] = []string{
						strconv.Itoa(a.ID),
						a.Filename,
						strconv.FormatInt(a.Filesize, 10),
						a.ContentType,
					}
				}
				printer.Table([]string{"ID", "Filename", "Size", "Content-Type"}, rows)
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&includes, "include", nil,
		"Include related data: journals, children, relations, attachments, watchers, changesets, allowed_statuses (Redmine 5.0+), reactions (Redmine 7.0+) (repeatable or comma-separated)")
	_ = cmd.RegisterFlagCompletionFunc("include", completeGetIncludes)
	cmd.Flags().BoolVar(&journals, "journals", false, "Include issue history/comments (shorthand for --include journals)")
	cmd.Flags().BoolVar(&children, "children", false, "Include child issues (shorthand for --include children)")
	cmd.Flags().BoolVar(&relations, "relations", false, "Include issue relations (shorthand for --include relations)")
	cmd.Flags().BoolVar(&attachments, "attachments", false, "Include the issue's attachments (shorthand for --include attachments)")
	cmd.Flags().StringVar(&downloadAttachments, "download-attachments", "", "Download every attachment of the issue into the given directory")
	cmd.Flags().BoolVar(&clobber, "clobber", false, "Overwrite existing files when using --download-attachments")
	cmd.Flags().BoolVar(&allowHidden, "allow-hidden", false, `With --download-attachments, allow saving attachments whose filename starts with "."`)
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}

// downloadIssueAttachments streams every attachment of the issue into dir,
// reporting progress to stderr (via the printer) so it never pollutes JSON
// output on stdout. Attachments that share a filename are given distinct
// names (see cmdutil.UniqueAttachmentFilenames) so none overwrites another.
// Every target is checked up front, so a conflict (a hidden filename without
// opts.AllowHidden, an existing file without opts.Clobber, or a directory)
// aborts the command before anything is downloaded.
func downloadIssueAttachments(ctx context.Context, client *api.Client, printer output.Printer, issue *models.Issue, dir string, opts cmdutil.SaveOptions) error {
	if len(issue.Attachments) == 0 {
		printer.Warning(fmt.Sprintf("Issue #%d has no attachments to download", issue.ID))
		return nil
	}
	names := cmdutil.UniqueAttachmentFilenames(issue.Attachments)
	for i := range issue.Attachments {
		att := &issue.Attachments[i]
		err := cmdutil.CheckHiddenFilename(dir, names[i], opts.AllowHidden)
		if err == nil {
			err = cmdutil.CheckDownloadTarget(filepath.Join(dir, names[i]), opts.Clobber)
		}
		if err != nil {
			return fmt.Errorf("downloading attachment %d (%s): %w", att.ID, att.Filename, err)
		}
	}
	for i := range issue.Attachments {
		att := &issue.Attachments[i]
		path, n, err := cmdutil.SaveAttachmentToDirAs(ctx, client, att, dir, names[i], opts)
		if err != nil {
			return fmt.Errorf("downloading attachment %d (%s): %w", att.ID, att.Filename, err)
		}
		printer.Success(fmt.Sprintf("Downloaded %q (%d bytes) to %s", att.Filename, n, path))
	}
	return nil
}
