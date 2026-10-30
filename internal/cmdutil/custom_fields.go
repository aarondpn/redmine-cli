package cmdutil

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
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

// CustomFieldPair is one --custom-field entry, in flag order.
type CustomFieldPair struct {
	Key   string
	Value string
}

// ParseCustomFieldPairs splits repeated --custom-field entries at the first
// '='. The key is trimmed; the value is kept verbatim, so spaces and further
// '=' characters survive. A missing '=' and an empty key are rejected.
func ParseCustomFieldPairs(raws []string) ([]CustomFieldPair, error) {
	pairs := make([]CustomFieldPair, 0, len(raws))
	for _, raw := range raws {
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--custom-field %q must be key=value", raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--custom-field %q: empty key", raw)
		}
		pairs = append(pairs, CustomFieldPair{Key: key, Value: val})
	}
	return pairs, nil
}

// customFieldKeyID returns the normalized decimal ID of a numeric key. ok is
// false for name keys.
func customFieldKeyID(key string) (id int, ok bool) {
	id, err := strconv.Atoi(key)
	return id, err == nil
}

// ParseCustomFieldValues parses repeated --custom-field key=value entries
// into the custom_field_values payload keyed by custom field ID.
//
// Numeric keys are used as IDs without a lookup. Other keys are resolved by
// name against /custom_fields.json (admin-only) in a single request, limited
// to fields of customizedType. A field given several times collects its
// values in flag order, which is how multi-value fields are written. A lone
// "key=" clears the field.
func ParseCustomFieldValues(ctx context.Context, client *api.Client, raws []string, customizedType string) (models.CustomFieldValues, error) {
	if len(raws) == 0 {
		return nil, nil
	}
	pairs, err := ParseCustomFieldPairs(raws)
	if err != nil {
		return nil, err
	}

	var nameKeys []string
	seen := make(map[string]bool)
	for _, p := range pairs {
		id, numeric := customFieldKeyID(p.Key)
		if numeric {
			if id <= 0 {
				return nil, fmt.Errorf("--custom-field %q: invalid custom field ID", p.Key+"="+p.Value)
			}
			continue
		}
		lower := strings.ToLower(p.Key)
		if !seen[lower] {
			seen[lower] = true
			nameKeys = append(nameKeys, p.Key)
		}
	}

	nameToField := make(map[string]models.CustomField, len(nameKeys))
	if len(nameKeys) > 0 {
		fields, err := resolver.ResolveCustomFieldsForType(ctx, client, nameKeys, customizedType)
		if err != nil {
			err = fmt.Errorf("resolve --custom-field keys: %w", err)
			if resolver.IsNameResolutionPermissionError(err) {
				err = WithHint(err, customFieldIDLookupHint(customizedType))
			}
			return nil, err
		}
		for i, name := range nameKeys {
			nameToField[strings.ToLower(name)] = fields[i]
		}
	}

	out := make(models.CustomFieldValues, len(pairs))
	defs := make(map[string]models.CustomField)
	for _, p := range pairs {
		var key string
		if id, numeric := customFieldKeyID(p.Key); numeric {
			key = strconv.Itoa(id)
		} else {
			def := nameToField[strings.ToLower(p.Key)]
			key = strconv.Itoa(def.ID)
			defs[key] = def
		}
		out[key] = append(out[key], p.Value)
	}

	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		vals := out[k]
		if len(vals) < 2 {
			continue
		}
		if slices.Contains(vals, "") {
			return nil, fmt.Errorf("--custom-field %s: an empty value clears the field and cannot be combined with other values", k)
		}
		if def, ok := defs[k]; ok && !def.Multiple {
			return nil, fmt.Errorf("custom field %q (ID %d) does not accept multiple values; pass it once", def.Name, def.ID)
		}
	}
	return out, nil
}

// customFieldRepeatKey normalizes a --custom-field key for comparing
// occurrences: numeric IDs by value, names case-insensitively.
func customFieldRepeatKey(key string) string {
	if id, ok := customFieldKeyID(key); ok {
		return strconv.Itoa(id)
	}
	return "name:" + strings.ToLower(key)
}

// HasRepeatedCustomFieldKey reports whether a field key occurs more than once
// in raws. Malformed entries report false.
func HasRepeatedCustomFieldKey(raws []string) bool {
	pairs, err := ParseCustomFieldPairs(raws)
	if err != nil {
		return false
	}
	seen := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		k := customFieldRepeatKey(p.Key)
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

// CustomFieldValidationHint explains the usual cause of a validation failure
// when a field was passed several times, and is empty otherwise.
func CustomFieldValidationHint(raws []string) string {
	if !HasRepeatedCustomFieldKey(raws) {
		return ""
	}
	return "A custom field given several --custom-field values must be a multi-value field; Redmine reports \"is invalid\" otherwise. " +
		"Pass single-value fields once (check the Multiple column of `redmine custom-fields list`, admin only)."
}

// WithValidationHint attaches the non-empty hints, joined by a space, to a
// validation (422) API error. Other errors pass through unchanged.
func WithValidationHint(err error, hints ...string) error {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || !apiErr.IsValidationError() {
		return err
	}
	hints = slices.DeleteFunc(slices.Clone(hints), func(h string) bool { return h == "" })
	return WithHint(err, strings.Join(hints, " "))
}

// customFieldIDLookupHint tells users who may not list custom field
// definitions where to read the numeric IDs instead.
func customFieldIDLookupHint(customizedType string) string {
	const lead = "Custom field definitions are admin-only. "
	switch customizedType {
	case models.CustomizedTypeIssue:
		return lead + "An issue lists its fields with IDs: `redmine issues get <id> --jq '.custom_fields'`."
	case models.CustomizedTypeProject:
		return lead + "A project lists its fields with IDs: `redmine projects get <identifier> --jq '.custom_fields'`."
	case models.CustomizedTypeTimeEntry:
		return lead + "A time entry lists its fields with IDs: `redmine time list --limit 1 --jq '.[0].custom_fields'`."
	case models.CustomizedTypeVersion:
		return lead + "A version lists its fields with IDs: `redmine versions get <id> --jq '.custom_fields'`."
	case models.CustomizedTypeUser:
		return lead + "Your account lists its fields with IDs: `redmine my-account get --jq '.custom_fields'`."
	case models.CustomizedTypeGroup:
		return lead + "A group lists its fields with IDs: `redmine groups get <id> --jq '.custom_fields'`."
	}
	return lead + "Use the numeric ID instead."
}

// AddCustomFieldFlag registers the repeatable --custom-field flag on cmd,
// with shell completion for fields of customizedType. label names the kind of
// field in the help text, for example "Version custom field".
func AddCustomFieldFlag(cmd *cobra.Command, f *Factory, target *[]string, label, customizedType string) {
	cmd.Flags().StringArrayVar(target, "custom-field", nil,
		label+" value as name=value or id=value (repeatable; repeat a field to set several values of a multi-value field, name= clears it)")
	_ = cmd.RegisterFlagCompletionFunc("custom-field", CompleteCustomFieldValues(f, customizedType))
}
