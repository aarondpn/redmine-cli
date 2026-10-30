package category

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdCategoryDelete(f *cmdutil.Factory) *cobra.Command {
	var (
		project    string
		reassignTo string
		force      bool
	)

	cmd := &cobra.Command{
		Use:     "delete <id-or-name>",
		Aliases: []string{"rm"},
		Short:   "Delete an issue category",
		Long: "Delete an issue category. Its issues are left without a category unless --reassign-to names " +
			"another category of the same project, which receives them. Accepts a numeric ID or category name " +
			"(uses the default project, or pass --project).",
		Example: `  # Delete a category (asks for confirmation)
  redmine categories delete 12

  # Move its issues to another category, skipping the confirmation
  redmine categories delete Backend --project myproject --reassign-to Platform --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			id, err := resolveCategoryID(ctx, f, client, args[0], project)
			if err != nil {
				return err
			}

			reassignID := 0
			if reassignTo != "" {
				if rid, convErr := strconv.Atoi(reassignTo); convErr == nil {
					reassignID = rid
				} else {
					// A name is looked up in the project of the category being
					// deleted, since both must belong to the same project.
					cat, err := ops.GetIssueCategory(ctx, client, ops.GetIssueCategoryInput{ID: id})
					if err != nil {
						return fmt.Errorf("failed to get category %d: %w", id, err)
					}
					scope := strconv.Itoa(cat.Project.ID)
					reassignID, err = resolver.ResolveCategory(ctx, client, reassignTo, scope)
					if err != nil {
						return err
					}
				}
				if reassignID <= 0 {
					return fmt.Errorf("invalid --reassign-to category ID %d", reassignID)
				}
				if reassignID == id {
					return fmt.Errorf("--reassign-to must differ from the category being deleted")
				}
			}

			printer := f.Printer("")
			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete issue category %d? Its issues will be left without a category.", id)
				if reassignID > 0 {
					msg = fmt.Sprintf("Are you sure you want to delete issue category %d? Its issues will be moved to category %d.", id, reassignID)
				}
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "category", id, "Delete cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Deleting category...")
			res, err := ops.DeleteIssueCategory(ctx, client, ops.DeleteIssueCategoryInput{ID: id, ReassignToID: reassignID})
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionDeleted, "category", id, res.Message)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (for name resolution; falls back to default project)")
	cmd.Flags().StringVar(&reassignTo, "reassign-to", "", "Move the category's issues to this category (ID or name) of the same project")
	cmdutil.AddForceFlag(cmd, &force)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("reassign-to", cmdutil.CompleteCategories(f))

	return cmd
}
