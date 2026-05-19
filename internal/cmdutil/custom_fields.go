package cmdutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// ParseCustomFieldValues parses repeated --custom-field key=value entries
// into the Redmine-expected map keyed by stringified custom-field ID.
// Numeric keys are passed through; non-numeric keys are resolved by name
// against /custom_fields.json (admin-only) via a single batch lookup.
//
// Shared by every resource that exposes a --custom-field flag (projects,
// issues, ...). Multi-value (Multiple=true) custom fields are not supported
// here yet — see issue tracker.
func ParseCustomFieldValues(ctx context.Context, client *api.Client, raws []string) (map[string]string, error) {
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
