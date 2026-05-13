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
			return fmt.Errorf("unknown --enable-module value %q (allowed: %s)", v, allowedModulesString())
		}
	}
	return nil
}

func allowedModulesString() string {
	out := make([]string, 0, len(allowedModules))
	for k := range allowedModules {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func completeEnabledModules(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	out := make([]string, 0, len(allowedModules))
	for k := range allowedModules {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}

func resolveTrackerNames(ctx context.Context, client *api.Client, values []string) ([]int, error) {
	if len(values) == 0 {
		return nil, nil
	}
	ids := make([]int, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		id, err := resolver.ResolveTracker(ctx, client, v)
		if err != nil {
			return nil, fmt.Errorf("resolve --tracker %q: %w", v, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func resolveCustomFieldNames(ctx context.Context, client *api.Client, values []string) ([]int, error) {
	if len(values) == 0 {
		return nil, nil
	}
	ids := make([]int, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		id, err := resolver.ResolveCustomField(ctx, client, v)
		if err != nil {
			return nil, fmt.Errorf("resolve --issue-custom-field %q: %w", v, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
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
	for _, raw := range raws {
		eq := strings.IndexByte(raw, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("--custom-field %q must be key=value", raw)
		}
		key := strings.TrimSpace(raw[:eq])
		val := raw[eq+1:]
		if key == "" {
			return nil, fmt.Errorf("--custom-field %q: empty key", raw)
		}

		if _, err := strconv.Atoi(key); err == nil {
			out[key] = val
			continue
		}

		id, err := resolver.ResolveCustomField(ctx, client, key)
		if err != nil {
			return nil, fmt.Errorf("resolve --custom-field key %q: %w", key, err)
		}
		out[strconv.Itoa(id)] = val
	}
	return out, nil
}
