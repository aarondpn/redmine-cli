package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdArchive(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "archive <identifier>",
		Short: "Archive a project",
		Long: "Archive a Redmine project. Archived projects are hidden from " +
			"default listings until unarchived. Requires Redmine 5.0 or newer.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")
			identifier := args[0]

			if !force {
				msg := fmt.Sprintf("Are you sure you want to archive project %q?", identifier)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionArchived, "project", identifier, "Archive cancelled")
					return nil
				}
			}

			if _, err := ops.ArchiveProject(context.Background(), client, ops.ArchiveProjectInput{Identifier: identifier}); err != nil {
				return err
			}

			printer.Action(output.ActionArchived, "project", identifier, fmt.Sprintf("Project %q archived", identifier))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
