package cmdutil

import (
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// FormatHours renders an hours value with two decimals.
func FormatHours(h float64) string {
	return fmt.Sprintf("%.2f", h)
}

// FormatCustomFieldValue renders one custom field value: a string as is, a
// multi-value list joined with ", ", nil as "".
func FormatCustomFieldValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []any:
		parts := make([]string, 0, len(val))
		for _, p := range val {
			if s := FormatCustomFieldValue(p); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(val, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// FormatCustomFields renders "Name: value; ..." for the custom fields that
// have a value. Fields with an empty value are skipped; it returns "" when
// none has one, so callers can omit the row.
func FormatCustomFields(items []models.CustomFieldValue) string {
	parts := make([]string, 0, len(items))
	for _, cf := range items {
		v := FormatCustomFieldValue(cf.Value)
		if v == "" {
			continue
		}
		parts = append(parts, cf.Name+": "+v)
	}
	return strings.Join(parts, "; ")
}
