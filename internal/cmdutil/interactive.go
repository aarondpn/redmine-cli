package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// PrepareInteractiveCommand opts a command out of structured output modes.
// It rejects an explicitly requested --output or --jq flag and otherwise
// forces the command to render using the default interactive/table path,
// ignoring any configured output_format.
func PrepareInteractiveCommand(cmd *cobra.Command, f *Factory) error {
	f.OutputFormat = output.FormatTable
	f.JQ = nil
	for _, name := range []string{"output", "jq"} {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return fmt.Errorf("--%s is not supported for %s", name, cmd.CommandPath())
		}
	}
	return nil
}
