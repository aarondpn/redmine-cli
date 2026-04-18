package auth

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdAuth creates the auth command group.
func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication profiles",
		Long:  "Login, logout, and switch between Redmine server profiles.",
	}

	cmd.AddCommand(NewCmdLogin(f))
	cmd.AddCommand(NewCmdLogout(f))
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdSwitch(f))
	cmd.AddCommand(NewCmdStatus(f))

	return cmd
}
