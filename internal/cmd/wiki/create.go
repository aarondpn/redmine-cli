package wiki

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

func newCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		text     string
		title    string
		parent   string
		comments string
		attach   []string
		format   string
	)

	cmd := &cobra.Command{
		Use:     "create <page>",
		Aliases: []string{"new"},
		Short:   "Create a wiki page",
		Long:    "Create a new Redmine wiki page (or overwrite an existing one).",
		Example: `  # Create a new wiki page
  redmine wiki create MyPage --project myproject --text "h1. Hello World"

  # Store the page under a different title, with a change comment
  redmine wiki create MyPage --project myproject --text "Content here" \
    --title "My Page" --comments "Initial draft"

  # Nest the new page under an existing page
  redmine wiki create Installation --project myproject \
    --text "h1. Installation" --parent "User Guide"

  # Read the page content from a file (or from stdin with "-")
  redmine wiki create MyPage --project myproject --text-file page.textile

  # Attach a file
  redmine wiki create MyPage --project myproject --text "See diagram" \
    --attach ./diagram.png`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)

			if _, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "text", "text-file", &text); err != nil {
				return err
			}
			if err := cmdutil.RejectEmptyTextFile(cmd, "text-file", text, ""); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			var uploads []models.Upload
			if len(attach) > 0 {
				uploads, err = cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
			}

			stop := printer.Spinner("Creating wiki page...")
			page, err := ops.CreateWikiPage(ctx, client, ops.CreateWikiPageInput{
				ProjectID:   projectID,
				Page:        args[0],
				Text:        text,
				Title:       title,
				ParentTitle: parent,
				Comments:    comments,
				Uploads:     uploads,
			})
			stop()
			if err != nil {
				if coded := parentNotAppliedError(err); coded != nil {
					return coded
				}
				return fmt.Errorf("failed to create wiki page %q: %w", args[0], err)
			}

			effective := args[0]
			if page.Title != "" {
				effective = page.Title
			}
			printer.Resource(page, fmt.Sprintf("Wiki page %q created", effective))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().StringVarP(&text, "text", "t", "", "Page content in Textile/Markdown (required unless --text-file is given)")
	cmdutil.AddTextFileFlag(cmd, "text", "text-file", "F")
	cmd.Flags().StringVar(&title, "title", "", "Page title to store the page under instead of <page>; Redmine uses it as the URL slug")
	cmd.Flags().StringVar(&parent, "parent", "", "Title of an existing page to nest this page under")
	cmd.Flags().StringVar(&comments, "comments", "", "Change comment")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")
	cmdutil.AddOutputFlag(cmd, &format)

	cmd.MarkFlagsOneRequired("text", "text-file")
	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
