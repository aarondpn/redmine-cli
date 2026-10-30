package models

// Version represents a Redmine project version (milestone).
type Version struct {
	ID            int    `json:"id"`
	Project       IDName `json:"project"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Status        string `json:"status"` // "open", "locked", "closed"
	DueDate       string `json:"due_date,omitempty"`
	Sharing       string `json:"sharing"`
	WikiPageTitle string `json:"wiki_page_title,omitempty"`
	CreatedOn     string `json:"created_on"`
	UpdatedOn     string `json:"updated_on"`

	// EstimatedHours and SpentHours are only sent by GET /versions/:id.json,
	// and only to callers who may view time entries.
	EstimatedHours *float64           `json:"estimated_hours,omitempty"`
	SpentHours     *float64           `json:"spent_hours,omitempty"`
	CustomFields   []CustomFieldValue `json:"custom_fields,omitzero"`
}

// VersionCreate defines fields for creating a project version.
type VersionCreate struct {
	Name              string            `json:"name"`
	Status            string            `json:"status,omitempty"`
	Sharing           string            `json:"sharing,omitempty"`
	DueDate           string            `json:"due_date,omitempty"`
	Description       string            `json:"description,omitempty"`
	WikiPageTitle     string            `json:"wiki_page_title,omitempty"`
	CustomFieldValues CustomFieldValues `json:"custom_field_values,omitempty"`
}

// VersionUpdate defines fields for updating a project version.
type VersionUpdate struct {
	Name              *string           `json:"name,omitempty"`
	Status            *string           `json:"status,omitempty"`
	Sharing           *string           `json:"sharing,omitempty"`
	DueDate           *string           `json:"due_date,omitempty"`
	Description       *string           `json:"description,omitempty"`
	WikiPageTitle     *string           `json:"wiki_page_title,omitempty"`
	CustomFieldValues CustomFieldValues `json:"custom_field_values,omitempty"`
}
