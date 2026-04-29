package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <identifier>",
		Aliases: []string{"rm"},
		Short:   "Delete a project",
		Long:    "Delete a Redmine project and all its data.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")
			identifier := args[0]

			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete project %q?", identifier)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "project", identifier, "Deletion cancelled")
					return nil
				}
			}

			if _, err := ops.DeleteProject(context.Background(), client, ops.DeleteProjectInput{Identifier: identifier}); err != nil {
				return err
			}

			printer.Action(output.ActionDeleted, "project", identifier, fmt.Sprintf("Project %q deleted", identifier))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
