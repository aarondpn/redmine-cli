package models

// Version represents a Redmine project version (milestone).
type Version struct {
	ID          int    `json:"id"`
	Project     IDName `json:"project"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"` // "open", "locked", "closed"
	DueDate     string `json:"due_date,omitempty"`
	Sharing     string `json:"sharing"`
	CreatedOn   string `json:"created_on"`
	UpdatedOn   string `json:"updated_on"`
}
