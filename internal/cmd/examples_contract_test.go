package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHelpExamplesUseExistingFlags pins that every `redmine ...` invocation
// in a command's Example block resolves to a real command and only uses long
// flags that command (or its parents) actually defines. Help examples are the
// primary documentation for agents, so a renamed or removed flag must fail
// here instead of silently teaching a broken recipe.
func TestHelpExamplesUseExistingFlags(t *testing.T) {
	root := NewRootCmd("test")

	var problems []string
	walk(root, func(c *cobra.Command) {
		for _, line := range exampleInvocations(c.Example) {
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[0] != "redmine" {
				continue
			}
			target, _, err := root.Find(fields[1:])
			if err != nil || target == root || !target.Runnable() {
				problems = append(problems, c.CommandPath()+": unknown or non-runnable command in example: "+line)
				continue
			}
			for _, field := range fields[1:] {
				if !strings.HasPrefix(field, "--") || field == "--" {
					continue
				}
				name := strings.TrimPrefix(field, "--")
				if i := strings.Index(name, "="); i >= 0 {
					name = name[:i]
				}
				if target.Flags().Lookup(name) == nil && target.InheritedFlags().Lookup(name) == nil {
					problems = append(problems, c.CommandPath()+": example uses unknown flag --"+name+" for "+target.CommandPath()+": "+line)
				}
			}
		}
	})

	if len(problems) > 0 {
		slices.Sort(problems)
		t.Fatalf("help examples reference commands or flags that do not exist:\n%s", strings.Join(problems, "\n"))
	}
}

// exampleInvocations extracts the command lines from a cobra Example block,
// skipping comments, joining backslash continuations, and cutting pipelines.
func exampleInvocations(example string) []string {
	var (
		out     []string
		current strings.Builder
	)
	for _, raw := range strings.Split(example, "\n") {
		line := strings.TrimSpace(raw)
		if current.Len() == 0 && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}
		continued := strings.HasSuffix(line, "\\")
		current.WriteString(strings.TrimSuffix(line, "\\"))
		current.WriteString(" ")
		if continued {
			continue
		}
		cmdLine := current.String()
		current.Reset()
		if i := strings.Index(cmdLine, "|"); i >= 0 {
			cmdLine = cmdLine[:i]
		}
		out = append(out, strings.TrimSpace(cmdLine))
	}
	return out
}
