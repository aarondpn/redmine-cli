package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
)

// Customized types reported by /custom_fields.json (customized_type).
const (
	CustomizedTypeIssue     = "issue"
	CustomizedTypeProject   = "project"
	CustomizedTypeTimeEntry = "time_entry"
	CustomizedTypeVersion   = "version"
	CustomizedTypeUser      = "user"
	CustomizedTypeGroup     = "group"
)

// CustomFieldInput is the value written for one custom field: one entry for a
// single-value field, several for a multi-value field.
//
// It is sent as a JSON string when it holds at most one value and as an array
// of strings from two values on. Redmine rejects an array on a single-value
// field ("is invalid") but accepts a plain string on both kinds, so the client
// never has to know whether a field is multi-value until it really sends
// several values. For the same reason clearing a field is always the empty
// string: [] and [""] are invalid on single-value fields.
type CustomFieldInput []string

// MarshalJSON implements json.Marshaler. Empty entries are dropped when
// several values are given, as Redmine does, and nothing at all clears the
// field.
func (v CustomFieldInput) MarshalJSON() ([]byte, error) {
	vals := slices.Clone([]string(v))
	if len(vals) > 1 {
		vals = slices.DeleteFunc(vals, func(s string) bool { return s == "" })
	}
	switch len(vals) {
	case 0:
		return []byte(`""`), nil
	case 1:
		return json.Marshal(vals[0])
	default:
		return json.Marshal(vals)
	}
}

// UnmarshalJSON implements json.Unmarshaler. It accepts a string or an array
// of strings and rejects everything else, matching the MCP input schema.
func (v *CustomFieldInput) UnmarshalJSON(data []byte) error {
	errInvalid := errors.New("custom field value must be a string or an array of strings")
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errInvalid
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return errInvalid
		}
		*v = CustomFieldInput{s}
	case '[':
		list := []string{}
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return errInvalid
		}
		*v = CustomFieldInput(list)
	default:
		return errInvalid
	}
	return nil
}

// CustomFieldValues is the custom_field_values payload shared by every
// writable resource, keyed by the custom field ID as a decimal string.
type CustomFieldValues map[string]CustomFieldInput
