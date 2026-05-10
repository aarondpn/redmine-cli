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
	)

	cmd := &cobra.Command{
		Use:     "update <page>",
		Aliases: []string{"edit"},
		Short:   "Update a wiki page",
		Long: "Update an existing Redmine wiki page.\n\n" +
			"To avoid silently overwriting concurrent edits, pass --expect-version " +
			"with the version you last fetched, or --ensure-current to refetch the " +
			"page and use its current version. Either flag turns 409 Conflict into a " +
			"clear error instead of a silent overwrite.",
		Example: `  # Update page content
  redmine wiki update MyPage --project myproject --text "Updated content"

  # Update with a change comment (text is preserved when omitted)
  redmine wiki update MyPage --project myproject --comments "Fixed typo"

  # Rename a page
  redmine wiki update MyPage --project myproject --title "New Title"

  # Attach a file
  redmine wiki update MyPage --project myproject --attach ./screenshot.png

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
				if errors.As(err, &apiErr) && apiErr.IsConflict() {
					expected := "(unspecified)"
					if input.Version != nil {
						expected = fmt.Sprintf("%d", *input.Version)
					}
					return fmt.Errorf("wiki page %q has been modified since version %s; refetch and retry: %w", args[0], expected, err)
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

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
