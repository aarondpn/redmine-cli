package models

// IDName represents a Redmine resource reference with id and name.
type IDName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// IDRef is a simple id-only reference (e.g., parent issue).
type IDRef struct {
	ID int `json:"id"`
}

// CustomFieldValue represents a custom field value on a resource.
type CustomFieldValue struct {
	ID       int         `json:"id"`
	Name     string      `json:"name"`
	Value    interface{} `json:"value"`
	Multiple bool        `json:"multiple,omitempty"`
}

// Journal represents an issue history entry.
type Journal struct {
	ID        int        `json:"id"`
	User      IDName     `json:"user"`
	Notes     string     `json:"notes"`
	CreatedOn string     `json:"created_on"`
	Details   []JournalDetail `json:"details,omitempty"`
}

// JournalDetail represents a single change in a journal entry.
type JournalDetail struct {
	Property string `json:"property"`
	Name     string `json:"name"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}
