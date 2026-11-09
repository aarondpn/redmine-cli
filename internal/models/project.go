package models

// ProjectFilter defines parameters for listing projects.
type ProjectFilter struct {
	// Status is sent as the status filter: a numeric code (1 active,
	// 5 closed, 9 archived) or several joined with "|". Empty sends nothing,
	// and Redmine 4.1+ then lists active projects only.
	Status   string
	Sort     string // e.g. "name:desc" (Redmine 4.1+)
	Includes []string
	// ExtraParams are raw Redmine filter parameters, applied last.
	ExtraParams FilterParams
	Limit       int
	Offset      int
}

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
	DefaultAssignee     *IDName            `json:"default_assignee,omitempty"`
	DefaultVersion      *IDName            `json:"default_version,omitempty"`
	Trackers            []IDName           `json:"trackers,omitzero"`
	IssueCategories     []IDName           `json:"issue_categories,omitzero"`
	EnabledModules      []IDName           `json:"enabled_modules,omitzero"`
	TimeEntryActivities []Enumeration      `json:"time_entry_activities,omitzero"`
	IssueCustomFields   []IDName           `json:"issue_custom_fields,omitzero"`
	CustomFields        []CustomFieldValue `json:"custom_fields,omitzero"`
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
	CustomFieldValues   CustomFieldValues `json:"custom_field_values,omitempty"`
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
	CustomFieldValues   CustomFieldValues `json:"custom_field_values,omitempty"`
}
