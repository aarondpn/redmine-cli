package models

// User represents a Redmine user.
type User struct {
	ID          int      `json:"id"`
	Login       string   `json:"login"`
	Admin       bool     `json:"admin"`
	FirstName   string   `json:"firstname"`
	LastName    string   `json:"lastname"`
	Mail        string   `json:"mail"`
	CreatedOn   string   `json:"created_on"`
	UpdatedOn   string   `json:"updated_on,omitempty"`
	LastLoginOn string   `json:"last_login_on,omitempty"`
	Status      int      `json:"status"`
	Memberships []IDName `json:"memberships,omitempty"`
}

// UserCreate defines fields for creating a user.
type UserCreate struct {
	Login     string `json:"login"`
	Password  string `json:"password"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Mail      string `json:"mail"`
	Admin     bool   `json:"admin,omitempty"`
}

// UserUpdate defines fields for updating a user.
type UserUpdate struct {
	FirstName *string `json:"firstname,omitempty"`
	LastName  *string `json:"lastname,omitempty"`
	Mail      *string `json:"mail,omitempty"`
	Admin     *bool   `json:"admin,omitempty"`
	Status    *int    `json:"status,omitempty"`
}

// UserFilter defines parameters for listing users.
type UserFilter struct {
	Status  string // "active", "registered", "locked"
	Name    string // filter by name
	GroupID int
	Limit   int
	Offset  int
}
