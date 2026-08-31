package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRunnableCommandsDeclarePositionalArgsPolicy pins that every runnable
// command declares an explicit cobra Args validator. Without one, cobra
// defaults to ArbitraryArgs and silently swallows stray positional
// arguments — e.g. `issues create --custom-field 2="a b" 61="c d"` used to
// drop the second pair without any error (issue #155). Commands that take
// no positionals must set cobra.NoArgs so users get a loud error instead.
func TestRunnableCommandsDeclarePositionalArgsPolicy(t *testing.T) {
	root := NewRootCmd("test")

	var offenders []string
	walk(root, func(c *cobra.Command) {
		if c.Runnable() && c.Args == nil {
			offenders = append(offenders, c.CommandPath())
		}
	})

	if len(offenders) > 0 {
		slices.Sort(offenders)
		t.Fatalf("runnable commands without an explicit Args validator (set cobra.NoArgs unless positionals are expected):\n%s", strings.Join(offenders, "\n"))
	}
}
