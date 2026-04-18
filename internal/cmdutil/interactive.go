package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// PrepareInteractiveCommand opts a command out of structured output modes.
// It rejects an explicitly requested --output flag and otherwise forces the
// command to render using the default interactive/table path, ignoring any
// configured output_format.
func PrepareInteractiveCommand(cmd *cobra.Command, f *Factory) error {
	if flag := cmd.Flags().Lookup("output"); flag != nil && flag.Changed {
		f.OutputFormat = output.FormatTable
		return fmt.Errorf("--output is not supported for %s", cmd.CommandPath())
	}

	f.OutputFormat = output.FormatTable
	return nil
}
