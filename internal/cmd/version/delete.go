package version

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdVersionDelete(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		force   bool
	)

	cmd := &cobra.Command{
		Use:     "delete <id-or-name>",
		Aliases: []string{"rm"},
		Short:   "Delete a project version",
		Long:    "Delete a Redmine project version. Accepts a numeric ID or version name (uses the default project, or pass --project).",
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

			printer := f.Printer("")
			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete version %d?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "version", id, "Delete cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Deleting version...")
			_, err = ops.DeleteVersion(ctx, client, ops.DeleteVersionInput{ID: id})
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionDeleted, "version", id, fmt.Sprintf("Deleted version %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (for name resolution; falls back to default project)")
	cmdutil.AddForceFlag(cmd, &force)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
