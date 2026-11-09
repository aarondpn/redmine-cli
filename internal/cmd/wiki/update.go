package wiki

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		project       string
		text          string
		title         string
		parent        string
		comments      string
		attach        []string
		expectVersion int
		ensureCurrent bool
		section       int
		sectionHash   string
		startPage     bool
		redirectLinks bool
		removeAttach  []int
		force         bool
	)

	cmd := &cobra.Command{
		Use:     "update <page>",
		Aliases: []string{"edit"},
		Short:   "Update a wiki page",
		Long: "Update an existing Redmine wiki page.\n\n" +
			"To avoid silently overwriting concurrent edits, pass --expect-version " +
			"with the version you last fetched, or --ensure-current to refetch the " +
			"page and use its current version. Either flag turns 409 Conflict into a " +
			"clear error instead of a silent overwrite.\n\n" +
			"To update only a single section of a page (based on heading position), " +
			"pass --section with the 1-based section number. Redmine replaces only " +
			"that section; the rest of the page is preserved. Use --section-hash to " +
			"add optimistic-locking at the section level.\n\n" +
			"To move a page under another page, pass --parent with the parent page " +
			"title; pass --parent \"\" to move it to the wiki root. Redmine only " +
			"applies this when you have the rename_wiki_pages permission, so the " +
			"page is read back afterwards and the command fails if the parent did " +
			"not change (or warns, if the parent may have been resolved through a " +
			"rename redirect).\n\n" +
			"A rename keeps a redirect from the old title; pass " +
			"--redirect-existing-links=false together with --title to drop it. " +
			"--start-page makes the page the wiki start page (needs the manage_wiki " +
			"permission). --remove-attachment deletes an attachment of the page by ID " +
			"(see 'redmine wiki get --include attachments') after asking for confirmation " +
			"unless --force is given. Redmine silently ignores these when you lack the permission.",
		Example: `  # Update page content
  redmine wiki update MyPage --project myproject --text "Updated content"

  # Replace the page content with a file (or stdin with "-")
  redmine wiki update MyPage --project myproject --text-file page.textile

  # Update with a change comment (text is preserved when omitted)
  redmine wiki update MyPage --project myproject --comments "Fixed typo"

  # Rename a page
  redmine wiki update MyPage --project myproject --title "New Title"

  # Rename a page without leaving a redirect from the old title
  redmine wiki update MyPage --project myproject --title "New Title" --redirect-existing-links=false

  # Make a page the wiki start page
  redmine wiki update MyPage --project myproject --start-page

  # Remove attachments (IDs from "wiki get --include attachments")
  redmine wiki update MyPage --project myproject --remove-attachment 12 --remove-attachment 15 --force

  # Move a page under another page
  redmine wiki update MyPage --project myproject --parent "User Guide"

  # Move a page back to the wiki root
  redmine wiki update MyPage --project myproject --parent ""

  # Attach a file
  redmine wiki update MyPage --project myproject --attach ./screenshot.png

  # Update only section 5 of the page
  redmine wiki update MyPage --project myproject \
    --section 5 --text "h3. Updated Section Content"

  # Section update with conflict detection
  redmine wiki update MyPage --project myproject \
    --section 5 --section-hash abc123 \
    --text "h3. Updated Section Content"

  # Optimistic concurrency: fail if the page has been edited since version 7
  redmine wiki update MyPage --project myproject \
    --text "Updated content" --expect-version 7

  # Convenience: refetch the page first and reuse its current version
  redmine wiki update MyPage --project myproject \
    --text "Updated content" --ensure-current`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)

			for _, id := range removeAttach {
				if id < 1 {
					return fmt.Errorf("invalid attachment ID %d: --remove-attachment takes positive attachment IDs", id)
				}
			}
			// The prompt reads standard input, which a "-" file flag consumes.
			if len(removeAttach) > 0 && !force && cmd.Flags().Changed("text-file") {
				if path, _ := cmd.Flags().GetString("text-file"); path == "-" {
					return fmt.Errorf("--text-file - reads standard input, so --remove-attachment cannot ask for confirmation: add --force")
				}
			}

			textSet, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "text", "text-file", &text)
			if err != nil {
				return err
			}
			if err := cmdutil.RejectEmptyTextFile(cmd, "text-file", text, ""); err != nil {
				return err
			}

			if cmd.Flags().Changed("expect-version") && ensureCurrent {
				return fmt.Errorf("--expect-version and --ensure-current are mutually exclusive")
			}
			if cmd.Flags().Changed("expect-version") && expectVersion < 1 {
				return fmt.Errorf("--expect-version must be >= 1")
			}
			if cmd.Flags().Changed("section") && section < 1 {
				return fmt.Errorf("--section must be >= 1")
			}
			// A section edit replaces the targeted section with --text. Without
			// --text the command would resend the whole current page body as
			// the section content, silently collapsing the page. Require it.
			if cmd.Flags().Changed("section") && !textSet {
				return fmt.Errorf("--section requires --text or --text-file")
			}
			if cmd.Flags().Changed("section-hash") && !cmd.Flags().Changed("section") {
				return fmt.Errorf("--section-hash requires --section")
			}

			if cmd.Flags().Changed("redirect-existing-links") && !cmd.Flags().Changed("title") {
				return fmt.Errorf("--redirect-existing-links only applies to a rename and requires --title")
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			input := ops.UpdateWikiPageInput{
				ProjectID: projectID,
				Page:      args[0],
			}

			// --ensure-current needs the current version. When --text is
			// omitted, ops.UpdateWikiPage fetches the page itself to resend
			// its text and pins the update to that version, which is what
			// --ensure-current asks for, so skip the extra fetch here.
			var current *models.WikiPage
			if ensureCurrent && textSet {
				current, err = ops.GetWikiPage(ctx, client, ops.GetWikiPageInput{
					ProjectID: projectID,
					Page:      args[0],
				})
				if err != nil {
					return fmt.Errorf("failed to fetch current wiki page %q: %w", args[0], err)
				}
			}

			if textSet {
				input.Text = &text
			}
			if cmd.Flags().Changed("title") {
				input.Title = &title
			}
			if cmd.Flags().Changed("parent") {
				input.ParentTitle = &parent
			}
			if cmd.Flags().Changed("comments") {
				input.Comments = &comments
			}
			input.IsStartPage = startPage
			if cmd.Flags().Changed("redirect-existing-links") {
				input.RedirectExistingLinks = &redirectLinks
			}
			input.DeletedAttachmentIDs = removeAttach
			if cmd.Flags().Changed("section") {
				s := section
				input.Section = &s
			}
			if cmd.Flags().Changed("section-hash") {
				input.SectionHash = &sectionHash
			}

			switch {
			case current != nil:
				v := current.Version
				input.Version = &v
			case cmd.Flags().Changed("expect-version"):
				v := expectVersion
				input.Version = &v
			}

			if len(attach) > 0 {
				uploads, err := cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
				input.Uploads = uploads
			}

			if len(removeAttach) > 0 && !force {
				msg := fmt.Sprintf("Remove %d attachment(s) from wiki page %q? This cannot be undone.", len(removeAttach), args[0])
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionUpdated, "wiki_page", args[0], "Update cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Updating wiki page...")
			result, err := ops.UpdateWikiPage(ctx, client, input)
			stop()
			if err != nil {
				if coded := parentNotAppliedError(err); coded != nil {
					return coded
				}
				var apiErr *api.APIError
				if errors.As(err, &apiErr) && apiErr.IsConflict() && input.Version != nil {
					// Surface wiki-specific context to the user. FormatError
					// and BuildErrorEnvelope render apiErr.Errors verbatim, so
					// prepending our note here ensures it reaches both the
					// human and JSON output paths instead of being shadowed
					// by a generic "Conflict" message.
					context := fmt.Sprintf("wiki page %q has been modified since version %d", args[0], *input.Version)
					// ops.UpdateWikiPage already says so when it caught the
					// stale version locally; don't repeat it.
					if len(apiErr.Errors) == 0 || !strings.HasPrefix(apiErr.Errors[0], context) {
						apiErr.Errors = append([]string{context}, apiErr.Errors...)
					}
					return err
				}
				return fmt.Errorf("failed to update wiki page %q: %w", args[0], err)
			}

			for _, w := range result.Warnings {
				printer.Warning(w)
			}
			printer.Action(output.ActionUpdated, "wiki_page", args[0], fmt.Sprintf("Wiki page %q updated", args[0]))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().StringVarP(&text, "text", "t", "", "Page content in Textile/Markdown")
	cmdutil.AddTextFileFlag(cmd, "text", "text-file", "F")
	cmd.Flags().StringVar(&title, "title", "", "New page title; renames the page (Redmine keeps a redirect from the old title)")
	cmd.Flags().StringVar(&parent, "parent", "", "Title of an existing page to move this page under. Pass empty to move it to the wiki root.")
	cmd.Flags().StringVar(&comments, "comments", "", "Change comment")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")
	cmd.Flags().IntVar(&expectVersion, "expect-version", 0, "Expected current page version. Server returns 409 Conflict if the page has moved on.")
	cmd.Flags().BoolVar(&ensureCurrent, "ensure-current", false, "Refetch the page first and assert its current version on update. Mutually exclusive with --expect-version.")
	cmd.Flags().IntVar(&section, "section", 0, "Section number (1-based) to update. When set, only that section is replaced; the rest of the page remains unchanged. Requires --text or --text-file.")
	cmd.Flags().StringVar(&sectionHash, "section-hash", "", "Hash of the original section content, used by Redmine for conflict detection on section edits. Only meaningful with --section.")

	cmd.Flags().BoolVar(&startPage, "start-page", false, "Make this page the wiki start page (needs the manage_wiki permission)")
	cmd.Flags().BoolVar(&redirectLinks, "redirect-existing-links", true, "On a rename, keep a redirect from the old title; use =false to drop it. Requires --title.")
	cmd.Flags().IntSliceVar(&removeAttach, "remove-attachment", nil, "Attachment ID to remove from the page (repeatable; asks for confirmation)")
	cmdutil.AddForceFlag(cmd, &force)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
