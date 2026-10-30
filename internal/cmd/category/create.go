package category

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdCategoryCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		name     string
		assignee string
		format   string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Args:    cobra.NoArgs,
		Short:   "Create an issue category",
		Long: "Create an issue category in a project. The optional default assignee (a user or group " +
			"that is a project member) is preselected for new issues in this category.",
		Example: `  # Create a category in the default project
  redmine categories create --name Backend

  # With a default assignee
  redmine categories create --project myproject --name Backend --assignee me

  # JSON output
  redmine categories create --project myproject --name Backend -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			if cmd.Flags().Changed("assignee") {
				if err := checkAssigneeFlag(assignee); err != nil {
					return err
				}
			}
			project, err = cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			input := ops.CreateIssueCategoryInput{ProjectID: project, Name: name}
			if assignee != "" && !resolver.IsNone(assignee) {
				aid, err := resolver.ResolveAssigneeInProject(ctx, client, assignee, project)
				if err != nil {
					return fmt.Errorf("resolving assignee: %w", err)
				}
				input.AssignedToID = aid
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Creating category...")
			category, err := ops.CreateIssueCategory(ctx, client, input)
			stop()
			if err != nil {
				return err
			}

			printer.Resource(category, fmt.Sprintf("Created category %q (ID: %d)", category.Name, category.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (falls back to default project)")
	cmd.Flags().StringVar(&name, "name", "", "Category name (required)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Default assignee: user or group name, login, ID, or 'me'")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteAssignees(f))

	return cmd
}
