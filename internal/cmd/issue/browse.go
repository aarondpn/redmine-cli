package issue

import (
	"context"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/tui"
	"github.com/spf13/cobra"
)

// NewCmdBrowse creates the issues browse command.
func NewCmdBrowse(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		status   string
		assignee string
	)

	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Interactive issue browser (TUI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				cfg, err := f.Config()
				if err == nil && cfg.DefaultProject != "" {
					project = cfg.DefaultProject
				}
			}

			if project != "" {
				resolvedProject, err := cmdutil.ResolveProjectID(context.Background(), f, project)
				if err != nil {
					return err
				}
				project = resolvedProject
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			stop := printer.Spinner("Loading issues...")
			issues, _, err := client.Issues.List(context.Background(), models.IssueFilter{
				ProjectID:    project,
				StatusID:     status,
				AssignedToID: assignee,
				Limit:        100,
			})
			stop()
			if err != nil {
				return err
			}

			return tui.RunBrowser(issues)
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Filter by project name, identifier, or ID")
	cmd.Flags().StringVar(&status, "status", "open", "Filter by status")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee")
	return cmd
}
