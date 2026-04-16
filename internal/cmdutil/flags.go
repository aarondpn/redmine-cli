package cmdutil

import (
	"github.com/spf13/cobra"
)

// AddPaginationFlags adds --limit and --offset flags to a command.
func AddPaginationFlags(cmd *cobra.Command, limit, offset *int) {
	cmd.Flags().IntVar(limit, "limit", 100, "Maximum number of results (0 for all)")
	cmd.Flags().IntVar(offset, "offset", 0, "Result offset for pagination")
}

// AddOutputFlag registers --output/-o as a local flag on the given command.
//
// The root command also registers --output/-o as a persistent flag, so every
// leaf command accepts it automatically. This helper is retained for
// backward compatibility with command handlers that want the flag's value
// bound to a local variable (and for unit tests that instantiate leaf
// commands in isolation, without the root). Local flags shadow the inherited
// persistent flag, so the end-user behavior is identical.
func AddOutputFlag(cmd *cobra.Command, format *string) {
	cmd.Flags().StringVarP(format, "output", "o", "", "Output format: table, wide, json, csv")
	_ = cmd.RegisterFlagCompletionFunc("output", CompleteOutputFormat)
}

// AddForceFlag adds the --force/-f flag to a command.
func AddForceFlag(cmd *cobra.Command, force *bool) {
	cmd.Flags().BoolVarP(force, "force", "f", false, "Skip confirmation prompt")
}
