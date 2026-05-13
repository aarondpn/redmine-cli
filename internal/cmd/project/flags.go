package project

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
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

// parseCustomFieldValues parses repeated --custom-field key=value entries
// into the Redmine-expected map keyed by stringified custom-field ID.
// Numeric keys are passed through; non-numeric keys are resolved by name
// against /custom_fields.json (admin-only).
func parseCustomFieldValues(ctx context.Context, client *api.Client, raws []string) (map[string]string, error) {
	if len(raws) == 0 {
		return nil, nil
	}

	out := make(map[string]string, len(raws))
	var nameKeys []string
	type pending struct {
		name string
		val  string
	}
	var deferred []pending

	for _, raw := range raws {
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--custom-field %q must be key=value", raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--custom-field %q: empty key", raw)
		}

		if _, err := strconv.Atoi(key); err == nil {
			out[key] = val
			continue
		}
		nameKeys = append(nameKeys, key)
		deferred = append(deferred, pending{name: key, val: val})
	}

	if len(nameKeys) == 0 {
		return out, nil
	}

	ids, err := resolver.ResolveCustomFieldNames(ctx, client, nameKeys)
	if err != nil {
		return nil, fmt.Errorf("resolve --custom-field keys: %w", err)
	}
	for i, p := range deferred {
		out[strconv.Itoa(ids[i])] = p.val
	}
	return out, nil
}
