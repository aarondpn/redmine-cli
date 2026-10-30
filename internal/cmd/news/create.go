package news

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project     string
		title       string
		summary     string
		description string
		attach      []string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Args:    cobra.NoArgs,
		Short:   "Create a news item",
		Long: "Create a news item in a project. Redmine answers without a body, so the new ID is " +
			"looked up afterwards (newest matching item by you); when it cannot be determined, " +
			"the command still succeeds, omits the ID and warns.",
		Example: `  # Create a news item
  redmine news create --project myproject --title "Release 2.0" \
    --summary "Now available" --description "We shipped 2.0 today."

  # Read the description from a file (or from stdin with "-")
  redmine news create --project myproject --title "Release 2.0" --description-file news.textile

  # Attach a file
  redmine news create --project myproject --title "Roadmap" \
    --description "See the attached roadmap." --attach ./roadmap.pdf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)

			if _, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "description", "description-file", &description); err != nil {
				return err
			}
			if err := cmdutil.RejectEmptyTextFile(cmd, "description-file", description, ""); err != nil {
				return err
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

			var uploads []models.Upload
			if len(attach) > 0 {
				uploads, err = cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
			}

			stop := printer.Spinner("Creating news...")
			result, err := ops.CreateNews(ctx, client, ops.CreateNewsInput{
				ProjectID:   projectID,
				Title:       title,
				Summary:     summary,
				Description: description,
				Uploads:     uploads,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to create news: %w", err)
			}

			var id any
			if result.ID > 0 {
				id = result.ID
			}
			printer.Action(output.ActionCreated, "news", id, result.Message)
			for _, w := range result.Warnings {
				printer.Warning(w)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (required if no default)")
	cmd.Flags().StringVar(&title, "title", "", "News title, max 60 characters (required)")
	cmd.Flags().StringVar(&summary, "summary", "", "Short summary, max 255 characters")
	cmd.Flags().StringVarP(&description, "description", "d", "", "News body (required unless --description-file is given)")
	cmdutil.AddTextFileFlag(cmd, "description", "description-file", "F")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")

	_ = cmd.MarkFlagRequired("title")
	cmd.MarkFlagsOneRequired("description", "description-file")
	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
