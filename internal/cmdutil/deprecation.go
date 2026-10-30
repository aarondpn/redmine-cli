package cmdutil

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// DeprecationDateLayout is the layout of the removeAfter date passed to
// DeprecateFlag.
const DeprecationDateLayout = "2006-01-02"

// DeprecationMarker is the prefix of the comment that must accompany every
// DeprecateFlag call so deprecated code can be found with a single grep.
const DeprecationMarker = "DEPRECATED(remove-after: "

// removeAfterAnnotation is the pflag annotation holding the removal date.
const removeAfterAnnotation = "redmine-cli-remove-after"

// DeprecateFlag marks the flag name on cmd as deprecated in favour of
// replacement (for example "--include users"). Cobra hides the flag from help
// and prints a warning to stderr when it is used, so machine-readable stdout
// stays clean. removeAfter is the last day (YYYY-MM-DD) the flag may still
// exist; the deprecation test in internal/cmd fails after that date.
//
// Every call site must carry a marker comment that names the flag as --name,
// so the code to delete can be found with
// `grep -rn "DEPRECATED(remove-after" internal`:
//
//	// DEPRECATED(remove-after: 2027-01-31): --include-users alias; delete the flag, the code reading it and its tests
//	cmdutil.DeprecateFlag(cmd, "include-users", "--include users", "2027-01-31")
//
// Tests and docs that reference a deprecated flag carry the same marker (a Go
// comment, or an MDX comment `{/* DEPRECATED(remove-after: ...): ... */}`), so
// the grep also lists everything that goes stale. The expiry test only scans
// non-test .go call sites.
//
// To remove a deprecated flag, delete the flag definition, the code that reads
// it, its tests, its docs, and every marker comment.
func DeprecateFlag(cmd *cobra.Command, name, replacement, removeAfter string) {
	if _, err := time.Parse(DeprecationDateLayout, removeAfter); err != nil {
		panic(fmt.Sprintf("DeprecateFlag(%s): removeAfter %q is not a YYYY-MM-DD date", name, removeAfter))
	}
	msg := fmt.Sprintf("use %s instead; will be removed after %s", replacement, removeAfter)
	if err := cmd.Flags().MarkDeprecated(name, msg); err != nil {
		panic(fmt.Sprintf("DeprecateFlag: %v", err))
	}
	if err := cmd.Flags().SetAnnotation(name, removeAfterAnnotation, []string{removeAfter}); err != nil {
		panic(fmt.Sprintf("DeprecateFlag: %v", err))
	}
}

// DeprecatedFlag describes a flag registered through DeprecateFlag.
type DeprecatedFlag struct {
	Command     string
	Name        string
	RemoveAfter string
}

// DeprecatedFlags returns every flag on cmd and its subcommands that was
// registered through DeprecateFlag.
func DeprecatedFlags(cmd *cobra.Command) []DeprecatedFlag {
	var out []DeprecatedFlag
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.Flags().VisitAll(func(fl *pflag.Flag) {
			if fl.Deprecated == "" {
				return
			}
			d := DeprecatedFlag{Command: c.CommandPath(), Name: fl.Name}
			if v := fl.Annotations[removeAfterAnnotation]; len(v) > 0 {
				d.RemoveAfter = v[0]
			}
			out = append(out, d)
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(cmd)
	return out
}
