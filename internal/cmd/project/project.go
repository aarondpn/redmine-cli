package project

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdProject creates the parent projects command.
func NewCmdProject(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Aliases: []string{"p"},
		Short:   "Manage Redmine projects",
		Long:    "List, view, create, update, and delete Redmine projects.",
	}

	cmd.AddCommand(newCmdList(f))
	cmd.AddCommand(newCmdGet(f))
	cmd.AddCommand(newCmdCreate(f))
	cmd.AddCommand(newCmdUpdate(f))
	cmd.AddCommand(newCmdDelete(f))
	cmd.AddCommand(newCmdMembers(f))

	return cmd
}
