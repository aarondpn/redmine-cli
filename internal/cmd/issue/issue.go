package issue

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

// assigneeName returns the name of the assignee or an empty string if nil.
func assigneeName(a *models.IDName) string {
	if a == nil {
		return ""
	}
	return a.Name
}

// NewCmdIssue creates the parent issues command.
func NewCmdIssue(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issues",
		Aliases: []string{"i"},
		Short:   "Manage Redmine issues",
		Long:    "List, view, create, update, and manage Redmine issues.",
	}

	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdGet(f))
	cmd.AddCommand(NewCmdCreate(f))
	cmd.AddCommand(NewCmdUpdate(f))
	cmd.AddCommand(NewCmdDelete(f))
	cmd.AddCommand(NewCmdAssign(f))
	cmd.AddCommand(NewCmdClose(f))
	cmd.AddCommand(NewCmdReopen(f))
	cmd.AddCommand(NewCmdComment(f))
	cmd.AddCommand(NewCmdBrowse(f))
	cmd.AddCommand(NewCmdOpen(f))

	return cmd
}
