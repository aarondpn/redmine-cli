package wiki

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

func newCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		text     string
		title    string
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

  # With display title and change comment
  redmine wiki create MyPage --project myproject --text "Content here" \
    --title "My Page" --comments "Initial draft"

  # Attach a file
  redmine wiki create MyPage --project myproject --text "See diagram" \
    --attach ./diagram.png`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			create := models.WikiPageCreate{
				Text: text,
			}
			if title != "" {
				create.Title = title
			}
			if comments != "" {
				create.Comments = comments
			}

			if len(attach) > 0 {
				uploads, err := cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
				create.Uploads = uploads
			}

			stop := printer.Spinner("Creating wiki page...")
			page, err := client.Wikis.Create(ctx, projectID, args[0], create)
			stop()
			if err != nil {
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
	cmd.Flags().StringVarP(&text, "text", "t", "", "Page content in Textile/Markdown (required)")
	cmd.Flags().StringVar(&title, "title", "", "Display title (defaults to page name)")
	cmd.Flags().StringVar(&comments, "comments", "", "Change comment")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("text")
	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
