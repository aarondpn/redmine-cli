package wiki

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

func newCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		text     string
		title    string
		comments string
	)

	cmd := &cobra.Command{
		Use:     "update <page>",
		Aliases: []string{"edit"},
		Short:   "Update a wiki page",
		Long:    "Update an existing Redmine wiki page.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			update := models.WikiPageUpdate{}

			if cmd.Flags().Changed("text") {
				update.Text = &text
			} else {
				// Redmine requires the text field on every PUT.
				// Fetch the current page and resend its text unchanged.
				current, err := client.Wikis.Get(ctx, projectID, args[0], nil)
				if err != nil {
					return err
				}
				update.Text = &current.Text
			}
			if cmd.Flags().Changed("title") {
				update.Title = &title
			}
			if cmd.Flags().Changed("comments") {
				update.Comments = &comments
			}

			err = client.Wikis.Update(ctx, projectID, args[0], update)
			if err != nil {
				return err
			}

			printer.Success(fmt.Sprintf("Wiki page %q updated", args[0]))
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project identifier or ID (required if no default)")
	cmd.Flags().StringVarP(&text, "text", "t", "", "Page content in Textile/Markdown")
	cmd.Flags().StringVar(&title, "title", "", "Display title")
	cmd.Flags().StringVar(&comments, "comments", "", "Change comment")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
