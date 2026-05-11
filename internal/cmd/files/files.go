// Package files implements the `redmine files` command group, which surfaces
// the Redmine REST Files API so users can list and upload project-level
// artifacts without dropping down to `redmine api`.
package files

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdFiles creates the parent files command.
func NewCmdFiles(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "files",
		Aliases: []string{"f"},
		Short:   "Manage Redmine project files",
		Long:    "List and upload Redmine project files (release artifacts and other attachments scoped to a project).",
	}

	cmd.AddCommand(newCmdList(f))
	cmd.AddCommand(newCmdUpload(f))

	return cmd
}
