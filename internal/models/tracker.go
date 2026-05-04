package models

// Tracker represents a Redmine tracker (e.g., Bug, Feature, Support).
type Tracker struct {
	ID                    int      `json:"id"`
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	DefaultStatus         *IDName  `json:"default_status,omitempty"`
	EnabledStandardFields []string `json:"enabled_standard_fields,omitempty"`
}
