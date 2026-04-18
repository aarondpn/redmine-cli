package group

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdGroup creates the parent groups command.
func NewCmdGroup(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "groups",
		Aliases: []string{"g"},
		Short:   "Manage groups",
		Long:    "List, view, create, update, and delete Redmine groups.",
	}

	cmd.AddCommand(newCmdGroupList(f))
	cmd.AddCommand(newCmdGroupGet(f))
	cmd.AddCommand(newCmdGroupCreate(f))
	cmd.AddCommand(newCmdGroupUpdate(f))
	cmd.AddCommand(newCmdGroupDelete(f))
	cmd.AddCommand(newCmdGroupAddUser(f))
	cmd.AddCommand(newCmdGroupRemoveUser(f))

	return cmd
}
