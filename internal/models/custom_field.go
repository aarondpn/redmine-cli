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
//
// IsForAll and Projects were added to the API in Redmine 7.0 (#44153). Roles
// is emitted for issue, time entry, project and version fields since 7.0
// (#44152); older versions restrict it to issue fields.
//
// The bool flags a server may omit are pointers so "not reported" stays
// distinguishable from "reported as false" - rendering them as "no" would
// claim a restriction the server never stated.
type CustomField struct {
	ID             int                        `json:"id"`
	Name           string                     `json:"name"`
	Description    string                     `json:"description,omitempty"`
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
	Editable       *bool                      `json:"editable,omitempty"`
	IsForAll       *bool                      `json:"is_for_all,omitempty"`
	PossibleValues []CustomFieldPossibleValue `json:"possible_values,omitempty"`
	Projects       []IDName                   `json:"projects,omitempty"`
	Trackers       []IDName                   `json:"trackers,omitempty"`
	Roles          []IDName                   `json:"roles,omitempty"`
}
