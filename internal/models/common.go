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
	ID           int    `json:"id"`
	User         IDName `json:"user"`
	Notes        string `json:"notes"`
	PrivateNotes bool   `json:"private_notes"`
	CreatedOn    string `json:"created_on"`
	// UpdatedOn is sent by Redmine 6.0+ for every journal and equals
	// CreatedOn until the notes are edited; UpdatedBy is the actual signal
	// that they were.
	UpdatedOn string          `json:"updated_on,omitempty"`
	UpdatedBy *IDName         `json:"updated_by,omitempty"`
	Details   []JournalDetail `json:"details,omitzero"`
	// Reactions (include=reactions) needs Redmine 7.1+.
	Reactions []Reaction `json:"reactions,omitzero"`
}

// Reaction is one reaction on an issue or journal (include=reactions,
// Redmine 7.1+).
type Reaction struct {
	User IDName `json:"user"`
}

// JournalDetail represents a single change in a journal entry.
type JournalDetail struct {
	Property string `json:"property"`
	Name     string `json:"name"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}
