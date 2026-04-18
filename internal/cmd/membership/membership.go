package membership

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdMemberships creates the parent memberships command.
func NewCmdMemberships(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memberships",
		Aliases: []string{"m"},
		Short:   "Manage project memberships",
		Long:    "List, view, create, update, and delete Redmine project memberships.",
	}

	cmd.AddCommand(newCmdMembershipList(f))
	cmd.AddCommand(newCmdMembershipGet(f))
	cmd.AddCommand(newCmdMembershipCreate(f))
	cmd.AddCommand(newCmdMembershipUpdate(f))
	cmd.AddCommand(newCmdMembershipDelete(f))

	return cmd
}
