// Package news implements the news command group.
package news

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdNews creates the news command group.
func NewCmdNews(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "news",
		Short: "Manage project news",
		Long: "List, view, create, update, and delete Redmine project news.\n\n" +
			"Use `redmine search news` for full-text search across news. News comments are " +
			"read-only over the REST API: `news get --include comments` shows them, but they " +
			"cannot be created or deleted from here.",
	}

	cmd.AddCommand(newCmdList(f))
	cmd.AddCommand(newCmdGet(f))
	cmd.AddCommand(newCmdCreate(f))
	cmd.AddCommand(newCmdUpdate(f))
	cmd.AddCommand(newCmdDelete(f))
	return cmd
}
