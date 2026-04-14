package wiki

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func newCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		force   bool
	)

	cmd := &cobra.Command{
		Use:     "delete <page>",
		Aliases: []string{"rm"},
		Short:   "Delete a wiki page",
		Long:    "Delete a Redmine wiki page.\n\nThis also removes all attachments and the page history.\nAny child pages will be re-parented to the wiki root.",
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

			pageTitle := args[0]

			if !force {
				msg := fmt.Sprintf("Delete wiki page %q?\nThis also removes all attachments and the page history. Any child pages will be re-parented to the wiki root.", pageTitle)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Warning("Deletion cancelled")
					return nil
				}
			}

			err = client.Wikis.Delete(ctx, projectID, pageTitle)
			if err != nil {
				return err
			}

			printer.Success(fmt.Sprintf("Wiki page %q deleted", pageTitle))
			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project identifier or ID (required if no default)")
	cmdutil.AddForceFlag(cmd, &force)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
