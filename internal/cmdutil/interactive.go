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
//
// The rejection is checked before the format is reset, so the factory still
// carries the requested format and the error is rendered accordingly: as the
// JSON error envelope for --jq or --output json, as text otherwise.
func PrepareInteractiveCommand(cmd *cobra.Command, f *Factory) error {
	for _, name := range []string{"output", "jq"} {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
			return fmt.Errorf("--%s is not supported for %s", name, cmd.CommandPath())
		}
	}
	f.OutputFormat = output.FormatTable
	f.JQ = nil
	return nil
}
