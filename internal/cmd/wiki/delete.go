package wiki

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		children string
		force    bool
	)

	cmd := &cobra.Command{
		Use:     "delete <page>",
		Aliases: []string{"rm"},
		Short:   "Delete a wiki page",
		Long: "Delete a Redmine wiki page.\n\nThis also removes all attachments and the page history.\n\n" +
			"By default any child pages are re-parented to the wiki root. Use --children to choose: " +
			"nullify (the default) or destroy (delete the whole subtree, including every descendant). " +
			"To keep the children under another page, move them first with " +
			"'redmine wiki update <child> --parent <page>' ('redmine wiki list' shows the parents).",
		Example: `  # Delete with confirmation prompt
  redmine wiki delete MyPage --project myproject

  # Skip confirmation
  redmine wiki delete MyPage --project myproject --force

  # Delete the page and all of its descendants
  redmine wiki delete MyPage --project myproject --children destroy --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)

			switch children {
			case "", ops.WikiTodoNullify, ops.WikiTodoDestroy:
			default:
				return fmt.Errorf("--children must be nullify or destroy, got %q", children)
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

			pageTitle := args[0]

			msg := fmt.Sprintf("Delete wiki page %q?\nThis also removes all attachments and the page history. Any child pages will be re-parented to the wiki root.", pageTitle)
			if children == ops.WikiTodoDestroy {
				msg = fmt.Sprintf("Delete wiki page %q and ALL of its descendant pages?\nThis also removes their attachments and page history.", pageTitle)
			}

			if !force {
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "wiki_page", pageTitle, "Deletion cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Deleting wiki page...")
			_, err = ops.DeleteWikiPage(ctx, client, ops.DeleteWikiPageInput{
				ProjectID: projectID,
				Page:      pageTitle,
				Todo:      children,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to delete wiki page %q: %w", pageTitle, err)
			}

			printer.Action(output.ActionDeleted, "wiki_page", pageTitle, fmt.Sprintf("Wiki page %q deleted", pageTitle))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().StringVar(&children, "children", "", "What to do with child pages: nullify (make them root pages, the default) or destroy (delete all descendants too)")
	cmdutil.AddForceFlag(cmd, &force)
	_ = cmd.RegisterFlagCompletionFunc("children", cobra.FixedCompletions(
		[]string{"nullify", "destroy"}, cobra.ShellCompDirectiveNoFileComp))

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
