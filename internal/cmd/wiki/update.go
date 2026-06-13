package wiki

import (
	"context"
	"errors"
	"fmt"

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
		comments      string
		attach        []string
		expectVersion int
		ensureCurrent bool
		section       int
		sectionHash   string
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
			"add optimistic-locking at the section level.",
		Example: `  # Update page content
  redmine wiki update MyPage --project myproject --text "Updated content"

  # Update with a change comment (text is preserved when omitted)
  redmine wiki update MyPage --project myproject --comments "Fixed typo"

  # Rename a page
  redmine wiki update MyPage --project myproject --title "New Title"

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
			ctx := context.Background()

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
			if cmd.Flags().Changed("section") && !cmd.Flags().Changed("text") {
				return fmt.Errorf("--section requires --text")
			}
			if cmd.Flags().Changed("section-hash") && !cmd.Flags().Changed("section") {
				return fmt.Errorf("--section-hash requires --section")
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

			// Fetch the current page when we either need its text (because
			// --text was omitted) or its version (because --ensure-current
			// was passed). One fetch covers both cases.
			needText := !cmd.Flags().Changed("text")
			var current *models.WikiPage
			if needText || ensureCurrent {
				current, err = ops.GetWikiPage(ctx, client, ops.GetWikiPageInput{
					ProjectID: projectID,
					Page:      args[0],
				})
				if err != nil {
					return fmt.Errorf("failed to fetch current wiki page %q: %w", args[0], err)
				}
			}

			if cmd.Flags().Changed("text") {
				input.Text = &text
			} else {
				// Redmine requires the text field on every PUT.
				// Resend the current text unchanged.
				input.Text = &current.Text
			}
			if cmd.Flags().Changed("title") {
				input.Title = &title
			}
			if cmd.Flags().Changed("comments") {
				input.Comments = &comments
			}
			if cmd.Flags().Changed("section") {
				s := section
				input.Section = &s
			}
			if cmd.Flags().Changed("section-hash") {
				input.SectionHash = &sectionHash
			}

			switch {
			case ensureCurrent:
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

			stop := printer.Spinner("Updating wiki page...")
			_, err = ops.UpdateWikiPage(ctx, client, input)
			stop()
			if err != nil {
				var apiErr *api.APIError
				if errors.As(err, &apiErr) && apiErr.IsConflict() && input.Version != nil {
					// Surface wiki-specific context to the user. FormatError
					// and BuildErrorEnvelope render apiErr.Errors verbatim, so
					// prepending our note here ensures it reaches both the
					// human and JSON output paths instead of being shadowed
					// by a generic "Conflict" message.
					context := fmt.Sprintf("wiki page %q has been modified since version %d", args[0], *input.Version)
					apiErr.Errors = append([]string{context}, apiErr.Errors...)
					return err
				}
				return fmt.Errorf("failed to update wiki page %q: %w", args[0], err)
			}

			printer.Action(output.ActionUpdated, "wiki_page", args[0], fmt.Sprintf("Wiki page %q updated", args[0]))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().StringVarP(&text, "text", "t", "", "Page content in Textile/Markdown")
	cmd.Flags().StringVar(&title, "title", "", "Display title")
	cmd.Flags().StringVar(&comments, "comments", "", "Change comment")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")
	cmd.Flags().IntVar(&expectVersion, "expect-version", 0, "Expected current page version. Server returns 409 Conflict if the page has moved on.")
	cmd.Flags().BoolVar(&ensureCurrent, "ensure-current", false, "Refetch the page first and assert its current version on update. Mutually exclusive with --expect-version.")
	cmd.Flags().IntVar(&section, "section", 0, "Section number (1-based) to update. When set, only that section is replaced; the rest of the page remains unchanged. Requires --text.")
	cmd.Flags().StringVar(&sectionHash, "section-hash", "", "Hash of the original section content, used by Redmine for conflict detection on section edits. Only meaningful with --section.")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
