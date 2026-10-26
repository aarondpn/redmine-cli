package cmdutil

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
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
	cmd.Flags().StringVarP(format, "output", "o", "", "Output format: table, json")
	_ = cmd.RegisterFlagCompletionFunc("output", CompleteOutputFormat)
}

// removedFormatCSV is the output format that was removed; jq's @csv
// replaces it.
const removedFormatCSV = "csv"

// isRemovedFormat reports whether format names the removed CSV format, in
// any letter case.
func isRemovedFormat(format string) bool {
	return strings.EqualFold(format, removedFormatCSV)
}

// csvReplacementHint tells users of the removed CSV format how to get CSV.
const csvReplacementHint = "use -o json with --jq and jq's @csv instead, e.g. redmine issues list -o json --jq '.[] | [.id, .subject] | @csv'"

// csvFallbackWarning is printed when a command renders with the table format
// because output_format still names the removed CSV format.
const csvFallbackWarning = "CSV output was removed, so output_format csv (profile or REDMINE_OUTPUT_FORMAT) falls back to table; " + csvReplacementHint

// CheckOutputFlag rejects an explicit --output csv (in any letter case) on
// cmd, the persistent flag or a leaf command's local one, before any request
// is made.
func CheckOutputFlag(cmd *cobra.Command) error {
	if of := cmd.Flags().Lookup("output"); of != nil && of.Changed && isRemovedFormat(of.Value.String()) {
		return &CodedError{
			Code:    output.ErrCodeValidationFailed,
			Message: "CSV output was removed; " + csvReplacementHint,
		}
	}
	return nil
}

// AddForceFlag adds the --force/-f flag to a command.
func AddForceFlag(cmd *cobra.Command, force *bool) {
	cmd.Flags().BoolVarP(force, "force", "f", false, "Skip confirmation prompt")
}

// annotationNoJSONOutput marks commands whose stdout is never JSON (shell
// scripts, protocol streams, human catalogs), so --jq can be rejected
// instead of being silently ignored.
const annotationNoJSONOutput = "redmine-cli/no-json-output"

// MarkNoJSONOutput declares that cmd never prints JSON to stdout.
func MarkNoJSONOutput(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationNoJSONOutput] = "true"
}

// HasNoJSONOutput reports whether cmd was marked with MarkNoJSONOutput.
func HasNoJSONOutput(cmd *cobra.Command) bool {
	return cmd.Annotations[annotationNoJSONOutput] == "true"
}
