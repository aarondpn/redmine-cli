package models

// Project represents a Redmine project.
type Project struct {
	ID                  int                `json:"id"`
	Name                string             `json:"name"`
	Identifier          string             `json:"identifier"`
	Description         string             `json:"description"`
	Homepage            string             `json:"homepage,omitempty"`
	Status              int                `json:"status"`
	IsPublic            bool               `json:"is_public"`
	InheritMembers      bool               `json:"inherit_members,omitempty"`
	Parent              *IDName            `json:"parent,omitempty"`
	CreatedOn           string             `json:"created_on"`
	UpdatedOn           string             `json:"updated_on"`
	DefaultAssignedTo   *IDName            `json:"default_assigned_to,omitempty"`
	DefaultVersion      *IDName            `json:"default_version,omitempty"`
	Trackers            []IDName           `json:"trackers,omitempty"`
	IssueCategories     []IDName           `json:"issue_categories,omitempty"`
	EnabledModules      []IDName           `json:"enabled_modules,omitempty"`
	TimeEntryActivities []Enumeration      `json:"time_entry_activities,omitempty"`
	IssueCustomFields   []IDName           `json:"issue_custom_fields,omitempty"`
	CustomFields        []CustomFieldValue `json:"custom_fields,omitempty"`
}

// ProjectCreate defines fields for creating a project.
type ProjectCreate struct {
	Name                string            `json:"name"`
	Identifier          string            `json:"identifier"`
	Description         string            `json:"description,omitempty"`
	Homepage            string            `json:"homepage,omitempty"`
	IsPublic            *bool             `json:"is_public,omitempty"`
	ParentID            int               `json:"parent_id,omitempty"`
	InheritMembers      bool              `json:"inherit_members,omitempty"`
	DefaultAssignedToID int               `json:"default_assigned_to_id,omitempty"`
	DefaultVersionID    int               `json:"default_version_id,omitempty"`
	TrackerIDs          []int             `json:"tracker_ids,omitempty"`
	EnabledModuleNames  []string          `json:"enabled_module_names,omitempty"`
	IssueCustomFieldIDs []int             `json:"issue_custom_field_ids,omitempty"`
	CustomFieldValues   map[string]string `json:"custom_field_values,omitempty"`
}

// ProjectUpdate defines fields for updating a project. Pointer-typed fields
// distinguish "not provided" (omit from request body) from "set to zero
// value". Slices use nil to mean "do not touch"; an explicitly empty slice
// would clear the field on Redmine.
type ProjectUpdate struct {
	Name                *string           `json:"name,omitempty"`
	Description         *string           `json:"description,omitempty"`
	Homepage            *string           `json:"homepage,omitempty"`
	IsPublic            *bool             `json:"is_public,omitempty"`
	ParentID            *int              `json:"parent_id,omitempty"`
	InheritMembers      *bool             `json:"inherit_members,omitempty"`
	DefaultAssignedToID *int              `json:"default_assigned_to_id,omitempty"`
	DefaultVersionID    *int              `json:"default_version_id,omitempty"`
	TrackerIDs          []int             `json:"tracker_ids,omitempty"`
	EnabledModuleNames  []string          `json:"enabled_module_names,omitempty"`
	IssueCustomFieldIDs []int             `json:"issue_custom_field_ids,omitempty"`
	CustomFieldValues   map[string]string `json:"custom_field_values,omitempty"`
}
