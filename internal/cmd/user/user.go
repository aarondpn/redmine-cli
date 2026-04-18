package user

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdUser creates the parent users command.
func NewCmdUser(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "users",
		Aliases: []string{"u"},
		Short:   "Manage users",
		Long:    "List, view, create, update, and delete Redmine users.",
	}

	cmd.AddCommand(newCmdUserList(f))
	cmd.AddCommand(newCmdUserGet(f))
	cmd.AddCommand(newCmdUserMe(f))
	cmd.AddCommand(newCmdUserCreate(f))
	cmd.AddCommand(newCmdUserUpdate(f))
	cmd.AddCommand(newCmdUserDelete(f))

	return cmd
}
