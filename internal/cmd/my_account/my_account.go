package myaccount

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdMyAccount creates the parent my-account command. It backs the
// /my/account.json endpoint, the only user-write path Redmine exposes to
// non-admins.
func NewCmdMyAccount(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "my-account",
		Short: "Manage your own Redmine account",
		Long:  "Inspect and update the authenticated user's account via /my/account.json. Works for non-admin users.",
	}

	cmd.AddCommand(newCmdMyAccountGet(f))
	cmd.AddCommand(newCmdMyAccountUpdate(f))

	return cmd
}
