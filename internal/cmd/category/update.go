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

func newCmdCategoryUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		name     string
		assignee string
	)

	cmd := &cobra.Command{
		Use:     "update <id-or-name>",
		Aliases: []string{"edit"},
		Short:   "Update an issue category",
		Long:    "Rename an issue category or change its default assignee ('none' clears it). Accepts a numeric ID or category name (uses the default project, or pass --project).",
		Example: `  # Rename a category
  redmine categories update 12 --name "Back end"

  # Change the default assignee by name, within the category's project
  redmine categories update Backend --project myproject --assignee "Backend Team"

  # Clear the default assignee
  redmine categories update 12 --assignee none`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("assignee") {
				return fmt.Errorf("nothing to update: provide --name and/or --assignee")
			}
			if cmd.Flags().Changed("assignee") {
				if err := checkAssigneeFlag(assignee); err != nil {
					return err
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			id, err := resolveCategoryID(ctx, f, client, args[0], project)
			if err != nil {
				return err
			}

			input := ops.UpdateIssueCategoryInput{ID: id}
			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if cmd.Flags().Changed("assignee") {
				aid := 0 // 'none' clears the default assignee
				if !resolver.IsNone(assignee) {
					// Names are matched against the category's project memberships.
					var projectRef string
					if resolver.NeedsPrincipalLookup(assignee) {
						cat, err := ops.GetIssueCategory(ctx, client, ops.GetIssueCategoryInput{ID: id})
						if err != nil {
							return fmt.Errorf("failed to fetch category for name resolution: %w", err)
						}
						projectRef = strconv.Itoa(cat.Project.ID)
					}
					aid, err = resolver.ResolveAssigneeInProject(ctx, client, assignee, projectRef)
					if err != nil {
						return fmt.Errorf("resolving assignee: %w", err)
					}
				}
				input.AssignedToID = &aid
			}

			printer := f.Printer("")
			stop := printer.Spinner("Updating category...")
			_, err = ops.UpdateIssueCategory(ctx, client, input)
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionUpdated, "category", id, fmt.Sprintf("Updated category %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (for name resolution; falls back to default project)")
	cmd.Flags().StringVar(&name, "name", "", "New category name")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Default assignee: user or group name, login, ID, 'me', or 'none' to clear")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteAssignees(f))

	return cmd
}
