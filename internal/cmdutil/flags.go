package cmdutil

import (
	"github.com/spf13/cobra"
)

// AddPaginationFlags adds --limit and --offset flags to a command.
func AddPaginationFlags(cmd *cobra.Command, limit, offset *int) {
	cmd.Flags().IntVar(limit, "limit", 25, "Maximum number of results")
	cmd.Flags().IntVar(offset, "offset", 0, "Result offset for pagination")
}

// AddOutputFlag adds the --output/-o flag to a command.
func AddOutputFlag(cmd *cobra.Command, format *string) {
	cmd.Flags().StringVarP(format, "output", "o", "", "Output format: table, wide, json, csv")
}

// AddForceFlag adds the --force/-f flag to a command.
func AddForceFlag(cmd *cobra.Command, force *bool) {
	cmd.Flags().BoolVarP(force, "force", "f", false, "Skip confirmation prompt")
}
