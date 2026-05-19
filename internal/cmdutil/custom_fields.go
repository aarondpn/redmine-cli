package cmdutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// ParseKeyValuePairs parses repeated `key=value` flag inputs into a map. The
// flagName is used to build clear error messages ("--filter %q must be
// key=value"). Empty keys and missing `=` are rejected.
func ParseKeyValuePairs(raws []string, flagName string) (map[string]string, error) {
	if len(raws) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(raws))
	for _, raw := range raws {
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--%s %q must be key=value", flagName, raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--%s %q: empty key", flagName, raw)
		}
		out[key] = val
	}
	return out, nil
}

// ParseCustomFieldValues parses repeated --custom-field key=value entries
// into the Redmine-expected map keyed by stringified custom-field ID.
// Numeric keys are passed through; non-numeric keys are resolved by name
// against /custom_fields.json (admin-only) via a single batch lookup.
//
// Multi-value (Multiple=true) custom fields are not supported here yet —
// see issue tracker.
func ParseCustomFieldValues(ctx context.Context, client *api.Client, raws []string) (map[string]string, error) {
	raw, err := ParseKeyValuePairs(raws, "custom-field")
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	out := make(map[string]string, len(raw))
	var nameKeys []string
	nameVals := make(map[string]string, len(raw))

	for key, val := range raw {
		if _, err := strconv.Atoi(key); err == nil {
			out[key] = val
			continue
		}
		nameKeys = append(nameKeys, key)
		nameVals[key] = val
	}

	if len(nameKeys) == 0 {
		return out, nil
	}

	ids, err := resolver.ResolveCustomFieldNames(ctx, client, nameKeys)
	if err != nil {
		return nil, fmt.Errorf("resolve --custom-field keys: %w", err)
	}
	for i, name := range nameKeys {
		out[strconv.Itoa(ids[i])] = nameVals[name]
	}
	return out, nil
}
