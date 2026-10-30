package project

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// allowedModules is the canonical list of Redmine project modules. Source:
// Redmine REST Projects API. Only these names are accepted by
// enabled_module_names on create/update.
var allowedModules = map[string]struct{}{
	"boards":         {},
	"calendar":       {},
	"documents":      {},
	"files":          {},
	"gantt":          {},
	"issue_tracking": {},
	"news":           {},
	"repository":     {},
	"time_tracking":  {},
	"wiki":           {},
}

func validateEnabledModules(values []string) error {
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := allowedModules[v]; !ok {
			return fmt.Errorf("unknown --enable-module value %q (allowed: %s)", v, sortedKeys(allowedModules))
		}
	}
	return nil
}

func completeEnabledModules(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return strings.Split(sortedKeys(allowedModules), ", "), cobra.ShellCompDirectiveNoFileComp
}

func sortedKeys(m map[string]struct{}) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
