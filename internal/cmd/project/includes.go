package project

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// allowedIncludes lists the include keys accepted by /projects.json and
// /projects/<id>.json. Keep alphabetical for stable error messages.
var allowedIncludes = map[string]struct{}{
	"enabled_modules":       {},
	"issue_categories":      {},
	"issue_custom_fields":   {},
	"time_entry_activities": {},
	"trackers":              {},
}

func validateProjectIncludes(values []string) error {
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := allowedIncludes[v]; !ok {
			return fmt.Errorf("unknown --include value %q (allowed: %s)", v, allowedIncludesString())
		}
	}
	return nil
}

func allowedIncludesString() string {
	out := make([]string, 0, len(allowedIncludes))
	for k := range allowedIncludes {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func completeProjectIncludes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	out := make([]string, 0, len(allowedIncludes))
	for k := range allowedIncludes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}
