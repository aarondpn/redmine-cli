package version

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdVersionUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		project       string
		name          string
		status        string
		sharing       string
		dueDate       string
		description   string
		wikiPageTitle string
	)

	cmd := &cobra.Command{
		Use:     "update <id-or-name>",
		Aliases: []string{"edit"},
		Short:   "Update a project version",
		Long:    "Update an existing Redmine project version. Accepts a numeric ID or version name (uses the default project, or pass --project).",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			id, err := resolveVersionID(ctx, f, client, args[0], project)
			if err != nil {
				return err
			}

			input := ops.UpdateVersionInput{ID: id}
			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if cmd.Flags().Changed("status") {
				input.Status = &status
			}
			if cmd.Flags().Changed("sharing") {
				input.Sharing = &sharing
			}
			if cmd.Flags().Changed("due-date") {
				resolved := cmdutil.ResolveDateKeyword(dueDate)
				input.DueDate = &resolved
			}
			if cmd.Flags().Changed("description") {
				input.Description = &description
			}
			if cmd.Flags().Changed("wiki-page-title") {
				input.WikiPageTitle = &wikiPageTitle
			}

			if _, err := ops.UpdateVersion(ctx, client, input); err != nil {
				return err
			}

			printer := f.Printer("")
			printer.Action(output.ActionUpdated, "version", id, fmt.Sprintf("Version %d updated", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (for name resolution; falls back to default project)")
	cmd.Flags().StringVar(&name, "name", "", "Version name")
	cmd.Flags().StringVar(&status, "status", "", "Version status: open, locked, closed")
	cmd.Flags().StringVar(&sharing, "sharing", "", "Version sharing: none, descendants, hierarchy, tree, system")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "Due date (YYYY-MM-DD or 'today')")
	cmd.Flags().StringVar(&description, "description", "", "Version description")
	cmd.Flags().StringVar(&wikiPageTitle, "wiki-page-title", "", "Wiki page title")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteVersionStatus)
	_ = cmd.RegisterFlagCompletionFunc("sharing", cmdutil.CompleteVersionSharing)

	return cmd
}
