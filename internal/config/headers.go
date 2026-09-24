package config

import (
	"fmt"
	"strings"
)

// ParseHeader splits a "Name: value" string into its header name and value.
func ParseHeader(s string) (string, string, error) {
	name, value, ok := strings.Cut(s, ":")
	name = strings.TrimSpace(name)
	if !ok || name == "" || strings.ContainsAny(name, " \t") {
		return "", "", fmt.Errorf("invalid header %q: expected \"Name: value\"", s)
	}
	return name, strings.TrimSpace(value), nil
}

// MergeHeaders returns base with the parsed "Name: value" overrides applied on
// top. Header names are matched case-insensitively, so an override replaces a
// base entry regardless of how either is capitalized.
func MergeHeaders(base map[string]string, overrides []string) (map[string]string, error) {
	if len(overrides) == 0 {
		return base, nil
	}
	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for _, h := range overrides {
		name, value, err := ParseHeader(h)
		if err != nil {
			return nil, err
		}
		for k := range merged {
			if strings.EqualFold(k, name) {
				delete(merged, k)
			}
		}
		merged[name] = value
	}
	return merged, nil
}
