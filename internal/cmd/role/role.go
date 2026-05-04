package role

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdRoles creates the roles command group.
func NewCmdRoles(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage roles",
		Long:  "List and inspect Redmine roles and their permissions.",
	}

	cmd.AddCommand(newCmdRoleList(f))
	cmd.AddCommand(newCmdRoleGet(f))

	return cmd
}
