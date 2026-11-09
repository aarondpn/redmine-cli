package models

import (
	"bytes"
	"encoding/json"
	"errors"
)

// FilterValues holds the values sent for one raw list-filter parameter. A
// key ending in "[]" (f[], v[status_id][]) carries one entry per value; any
// other key normally carries one.
type FilterValues []string

// MarshalJSON implements json.Marshaler: one value is a string, any other
// count an array, mirroring what UnmarshalJSON accepts.
func (v FilterValues) MarshalJSON() ([]byte, error) {
	if len(v) == 1 {
		return json.Marshal(v[0])
	}
	if v == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(v))
}

// UnmarshalJSON implements json.Unmarshaler. It accepts a string or an array
// of strings and rejects everything else, matching the MCP input schema.
func (v *FilterValues) UnmarshalJSON(data []byte) error {
	errInvalid := errors.New("filter value must be a string or an array of strings")
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
		*v = FilterValues{s}
	case '[':
		list := []string{}
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return errInvalid
		}
		*v = FilterValues(list)
	default:
		return errInvalid
	}
	return nil
}

// FilterParams are raw Redmine query-string parameters passed through to a
// list endpoint unchanged (the --filter / extra_params escape hatch), e.g.
// created_on=">=2025-01-01", cf_5="Critical" or f[]=["status_id","spent_on"].
//
// They are applied after the parameters built from dedicated filters, so a
// key that a dedicated filter also sets is replaced by the raw value.
type FilterParams map[string]FilterValues
