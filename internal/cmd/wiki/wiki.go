package wiki

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdWiki creates the parent wiki command.
func NewCmdWiki(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wiki",
		Aliases: []string{"w"},
		Short:   "Manage Redmine wiki pages",
		Long:    "List, view, create, update, and delete Redmine wiki pages.",
	}

	cmd.AddCommand(newCmdList(f))
	cmd.AddCommand(newCmdGet(f))
	cmd.AddCommand(newCmdCreate(f))
	cmd.AddCommand(newCmdUpdate(f))
	cmd.AddCommand(newCmdDelete(f))

	return cmd
}
