package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdUnarchive(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unarchive <identifier>",
		Short: "Unarchive a project",
		Long:  "Restore a previously archived Redmine project. Requires Redmine 5.0 or newer.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")
			identifier := args[0]

			if _, err := ops.UnarchiveProject(context.Background(), client, ops.UnarchiveProjectInput{Identifier: identifier}); err != nil {
				return err
			}

			printer.Action(output.ActionUnarchived, "project", identifier, fmt.Sprintf("Project %q unarchived", identifier))
			return nil
		},
	}

	return cmd
}
