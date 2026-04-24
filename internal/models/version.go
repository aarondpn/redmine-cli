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
}

// VersionCreate defines fields for creating a project version.
type VersionCreate struct {
	Name          string `json:"name"`
	Status        string `json:"status,omitempty"`
	Sharing       string `json:"sharing,omitempty"`
	DueDate       string `json:"due_date,omitempty"`
	Description   string `json:"description,omitempty"`
	WikiPageTitle string `json:"wiki_page_title,omitempty"`
}

// VersionUpdate defines fields for updating a project version.
type VersionUpdate struct {
	Name          *string `json:"name,omitempty"`
	Status        *string `json:"status,omitempty"`
	Sharing       *string `json:"sharing,omitempty"`
	DueDate       *string `json:"due_date,omitempty"`
	Description   *string `json:"description,omitempty"`
	WikiPageTitle *string `json:"wiki_page_title,omitempty"`
}
