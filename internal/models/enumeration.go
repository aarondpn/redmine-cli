package models

// Enumeration represents a Redmine enumeration (activity, priority, etc.).
type Enumeration struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Active    bool   `json:"active"`
}
