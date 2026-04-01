package issue

import (
	"context"
	"fmt"

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
			ctx := context.Background()
			project, err := cmdutil.DefaultProjectID(ctx, f, project)
			if err != nil {
				return err
			}

			cfg, err := f.Config()
			if err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			resolvedStatus, err := resolveIssueStatusFilter(ctx, client, status)
			if err != nil {
				return err
			}
			resolvedAssignee, err := resolveIssueAssigneeFilter(ctx, client, assignee, printer)
			if err != nil {
				return err
			}

			stop := printer.Spinner("Loading issues...")
			issues, _, err := client.Issues.List(ctx, models.IssueFilter{
				ProjectID:    project,
				StatusID:     resolvedStatus,
				AssignedToID: resolvedAssignee,
				Limit:        100,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to browse issues: %w", err)
			}

			return tui.RunBrowser(issues, cfg.Server)
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Filter by project name, identifier, or ID")
	cmd.Flags().StringVar(&status, "status", "open", "Filter by status: open, closed, *, status name, or ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assignee: me, name, login, or ID")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteIssueListStatus(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteUsers(f))

	return cmd
}
