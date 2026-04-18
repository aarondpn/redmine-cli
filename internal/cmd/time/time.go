package time

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdTime creates the parent time command.
func NewCmdTime(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "time",
		Aliases: []string{"t"},
		Short:   "Manage time entries",
		Long:    "Create, view, update, and delete time entries in Redmine.",
	}

	cmd.AddCommand(newCmdTimeList(f))
	cmd.AddCommand(newCmdTimeGet(f))
	cmd.AddCommand(newCmdTimeLog(f))
	cmd.AddCommand(newCmdTimeUpdate(f))
	cmd.AddCommand(newCmdTimeDelete(f))
	cmd.AddCommand(newCmdTimeSummary(f))

	return cmd
}
