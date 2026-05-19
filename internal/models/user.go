package models

// User represents a Redmine user.
type User struct {
	ID               int                `json:"id"`
	Login            string             `json:"login"`
	Admin            bool               `json:"admin"`
	FirstName        string             `json:"firstname"`
	LastName         string             `json:"lastname"`
	Mail             string             `json:"mail"`
	CreatedOn        string             `json:"created_on"`
	UpdatedOn        string             `json:"updated_on,omitempty"`
	LastLoginOn      string             `json:"last_login_on,omitempty"`
	Status           int                `json:"status"`
	APIKey           string             `json:"api_key,omitempty"`
	MailNotification string             `json:"mail_notification,omitempty"`
	MustChangePasswd *bool              `json:"must_change_passwd,omitempty"`
	Memberships      []UserMembership   `json:"memberships,omitempty"`
	Groups           []IDName           `json:"groups,omitempty"`
	CustomFields     []CustomFieldValue `json:"custom_fields,omitempty"`
}

// UserMembership represents one row of a user's project memberships as
// returned by GET /users/:id.json?include=memberships. The project + roles
// shape mirrors the Redmine REST response.
type UserMembership struct {
	ID      int      `json:"id"`
	Project IDName   `json:"project"`
	Roles   []IDName `json:"roles,omitempty"`
}

// UserCreate defines fields for creating a user. The send_information sibling
// flag on POST /users.json is passed separately through the API layer.
type UserCreate struct {
	Login            string  `json:"login"`
	Password         string  `json:"password,omitempty"`
	FirstName        string  `json:"firstname"`
	LastName         string  `json:"lastname"`
	Mail             string  `json:"mail"`
	Admin            bool    `json:"admin,omitempty"`
	MailNotification *string `json:"mail_notification,omitempty"`
	MustChangePasswd *bool   `json:"must_change_passwd,omitempty"`
	GeneratePassword *bool   `json:"generate_password,omitempty"`
	AuthSourceID     *int    `json:"auth_source_id,omitempty"`
}

// UserUpdate defines fields for updating a user.
type UserUpdate struct {
	FirstName        *string `json:"firstname,omitempty"`
	LastName         *string `json:"lastname,omitempty"`
	Mail             *string `json:"mail,omitempty"`
	Admin            *bool   `json:"admin,omitempty"`
	Status           *int    `json:"status,omitempty"`
	MailNotification *string `json:"mail_notification,omitempty"`
	MustChangePasswd *bool   `json:"must_change_passwd,omitempty"`
	GeneratePassword *bool   `json:"generate_password,omitempty"`
	AuthSourceID     *int    `json:"auth_source_id,omitempty"`
}

// MyAccountUpdate is the writable subset for PUT /my/account.json. The endpoint
// is the only user-write path available to non-admins; we intentionally keep
// the type narrow so admin-only fields cannot be sent here by accident.
type MyAccountUpdate struct {
	FirstName        *string `json:"firstname,omitempty"`
	LastName         *string `json:"lastname,omitempty"`
	Mail             *string `json:"mail,omitempty"`
	MailNotification *string `json:"mail_notification,omitempty"`
}

// UserFilter defines parameters for listing users.
type UserFilter struct {
	Status  string // "active", "registered", "locked"
	Name    string // filter by name
	GroupID int
	Limit   int
	Offset  int
}
