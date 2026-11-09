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
		project    string
		children   string
		reassignTo string
		force      bool
	)

	cmd := &cobra.Command{
		Use:     "delete <page>",
		Aliases: []string{"rm"},
		Short:   "Delete a wiki page",
		Long: "Delete a Redmine wiki page.\n\nThis also removes all attachments and the page history.\n\n" +
			"By default any child pages are re-parented to the wiki root. Use --children to choose: " +
			"nullify (the default), destroy (delete the whole subtree, including every descendant) or " +
			"reassign (move the children under another page named by --reassign-to; passing " +
			"--reassign-to alone implies reassign). Reassigning moves each child separately through " +
			"parent_title, so it needs the rename_wiki_pages permission and is not atomic: the " +
			"page is only deleted after every child has moved.",
		Example: `  # Delete with confirmation prompt
  redmine wiki delete MyPage --project myproject

  # Skip confirmation
  redmine wiki delete MyPage --project myproject --force

  # Delete the page and all of its descendants
  redmine wiki delete MyPage --project myproject --children destroy --force

  # Move the children under another page, then delete
  redmine wiki delete MyPage --project myproject --reassign-to "User Guide" --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)

			todo, err := resolveChildrenFlag(children, reassignTo)
			if err != nil {
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

			pageTitle := args[0]

			msg := fmt.Sprintf("Delete wiki page %q?\nThis also removes all attachments and the page history. Any child pages will be re-parented to the wiki root.", pageTitle)
			switch todo {
			case ops.WikiTodoDestroy:
				msg = fmt.Sprintf("Delete wiki page %q and ALL of its descendant pages?\nThis also removes their attachments and page history.", pageTitle)
			case ops.WikiTodoReassign:
				msg = fmt.Sprintf("Delete wiki page %q?\nThis also removes all attachments and the page history. Its child pages will be moved under %q.", pageTitle, reassignTo)
			}

			if !force {
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "wiki_page", pageTitle, "Deletion cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Deleting wiki page...")
			_, err = ops.DeleteWikiPage(ctx, client, ops.DeleteWikiPageInput{
				ProjectID:  projectID,
				Page:       pageTitle,
				Todo:       todo,
				ReassignTo: reassignTo,
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
	cmd.Flags().StringVar(&children, "children", "", "What to do with child pages: nullify (make them root pages, the default), destroy (delete all descendants too) or reassign (needs --reassign-to)")
	cmd.Flags().StringVar(&reassignTo, "reassign-to", "", "Title of an existing page that receives the children; implies --children reassign")
	cmdutil.AddForceFlag(cmd, &force)
	_ = cmd.RegisterFlagCompletionFunc("children", cobra.FixedCompletions(
		[]string{"nullify", "destroy", "reassign"}, cobra.ShellCompDirectiveNoFileComp))

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}

// resolveChildrenFlag validates --children and --reassign-to together and
// returns the todo to send ("" when neither flag was given).
func resolveChildrenFlag(children, reassignTo string) (string, error) {
	switch children {
	case "", ops.WikiTodoNullify, ops.WikiTodoDestroy, ops.WikiTodoReassign:
	default:
		return "", fmt.Errorf("--children must be one of nullify, destroy or reassign, got %q", children)
	}
	if reassignTo != "" {
		if children != "" && children != ops.WikiTodoReassign {
			return "", fmt.Errorf("--reassign-to cannot be combined with --children %s", children)
		}
		return ops.WikiTodoReassign, nil
	}
	if children == ops.WikiTodoReassign {
		return "", fmt.Errorf("--children reassign requires --reassign-to")
	}
	return children, nil
}
