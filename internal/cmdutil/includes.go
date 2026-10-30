package cmdutil

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ValidateIncludes rejects any --include value that is not in allowed. Blank
// entries are ignored.
func ValidateIncludes(values []string, allowed map[string]struct{}) error {
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := allowed[v]; !ok {
			return fmt.Errorf("unknown --include value %q (allowed: %s)", v, SortedIncludeKeys(allowed))
		}
	}
	return nil
}

// CompleteIncludes returns a shell completion function offering the allowed
// --include values.
func CompleteIncludes(allowed map[string]struct{}) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return strings.Split(SortedIncludeKeys(allowed), ", "), cobra.ShellCompDirectiveNoFileComp
	}
}

// SortedIncludeKeys renders the allowed include keys alphabetically so error
// messages stay stable.
func SortedIncludeKeys(m map[string]struct{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// MergeIncludes appends extra to includes, trimming whitespace and dropping
// blanks and duplicates while preserving first-seen order.
func MergeIncludes(includes []string, extra ...string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, v := range append(append([]string{}, includes...), extra...) {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
