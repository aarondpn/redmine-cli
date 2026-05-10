package models

// CustomFieldPossibleValue represents one entry of a list-format custom
// field's possible values. Redmine returns the field as an array of
// `{value,label}` objects, sometimes as bare strings on older versions; the
// list-of-objects form is what /custom_fields.json emits in 4.x+.
type CustomFieldPossibleValue struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// CustomField represents a Redmine custom field definition as returned by
// /custom_fields.json. The endpoint is admin-only.
type CustomField struct {
	ID             int                        `json:"id"`
	Name           string                     `json:"name"`
	CustomizedType string                     `json:"customized_type"`
	FieldFormat    string                     `json:"field_format"`
	Regexp         string                     `json:"regexp,omitempty"`
	MinLength      *int                       `json:"min_length,omitempty"`
	MaxLength      *int                       `json:"max_length,omitempty"`
	IsRequired     bool                       `json:"is_required"`
	IsFilter       bool                       `json:"is_filter"`
	Searchable     bool                       `json:"searchable"`
	Multiple       bool                       `json:"multiple"`
	DefaultValue   string                     `json:"default_value,omitempty"`
	Visible        bool                       `json:"visible"`
	PossibleValues []CustomFieldPossibleValue `json:"possible_values,omitempty"`
	Trackers       []IDName                   `json:"trackers,omitempty"`
	Roles          []IDName                   `json:"roles,omitempty"`
}
