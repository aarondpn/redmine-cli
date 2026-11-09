package models

// User represents a Redmine user.
type User struct {
	ID               int                `json:"id"`
	Login            string             `json:"login"`
	Admin            *bool              `json:"admin,omitempty"`
	FirstName        string             `json:"firstname"`
	LastName         string             `json:"lastname"`
	Mail             string             `json:"mail"`
	CreatedOn        string             `json:"created_on"`
	UpdatedOn        string             `json:"updated_on,omitempty"`
	LastLoginOn      string             `json:"last_login_on,omitempty"`
	PasswdChangedOn  string             `json:"passwd_changed_on,omitempty"`
	TwofaScheme      string             `json:"twofa_scheme,omitempty"`
	AvatarURL        string             `json:"avatar_url,omitempty"`  // Redmine 5.0+, only when an avatar exists
	AuthSource       *IDName            `json:"auth_source,omitempty"` // include=auth_source, Redmine 6.0+, admin only
	Status           *int               `json:"status,omitempty"`
	APIKey           string             `json:"api_key,omitempty"`
	MailNotification string             `json:"mail_notification,omitempty"`
	MustChangePasswd *bool              `json:"must_change_passwd,omitempty"`
	Memberships      []UserMembership   `json:"memberships,omitzero"`
	Groups           []IDName           `json:"groups,omitzero"`
	CustomFields     []CustomFieldValue `json:"custom_fields,omitzero"`
}

// UserMembership represents one row of a user's or group's project
// memberships as returned by GET /users/:id.json?include=memberships and
// GET /groups/:id.json?include=memberships. The project + roles shape mirrors
// the Redmine REST response.
type UserMembership struct {
	ID      int              `json:"id"`
	Project IDName           `json:"project"`
	Roles   []MembershipRole `json:"roles,omitzero"`
}

// UserPreferences holds user preference keys (hide_mail, time_zone,
// no_self_notified, ...) with their form values. Redmine reads them from a
// "pref" object next to "user", so the API layer sends them there; unknown
// keys are ignored by Redmine.
type UserPreferences map[string]string

// UserCreate defines fields for creating a user. The send_information sibling
// flag on POST /users.json is passed separately through the API layer, and
// Pref is sent as the top-level "pref" object.
type UserCreate struct {
	Login             string            `json:"login"`
	Password          string            `json:"password,omitempty"`
	FirstName         string            `json:"firstname"`
	LastName          string            `json:"lastname"`
	Mail              string            `json:"mail"`
	Admin             bool              `json:"admin,omitempty"`
	Status            *int              `json:"status,omitempty"`
	Language          *string           `json:"language,omitempty"`
	MailNotification  *string           `json:"mail_notification,omitempty"`
	MustChangePasswd  *bool             `json:"must_change_passwd,omitempty"`
	GeneratePassword  *bool             `json:"generate_password,omitempty"`
	AuthSourceID      *int              `json:"auth_source_id,omitempty"`
	CustomFieldValues CustomFieldValues `json:"custom_field_values,omitempty"`
	Pref              UserPreferences   `json:"-"`
}

// UserUpdate defines fields for updating a user. Pref is sent as the
// top-level "pref" object. GroupIDs and NotifiedProjectIDs replace the whole
// set; a non-nil empty slice clears it.
type UserUpdate struct {
	FirstName            *string           `json:"firstname,omitempty"`
	LastName             *string           `json:"lastname,omitempty"`
	Mail                 *string           `json:"mail,omitempty"`
	Admin                *bool             `json:"admin,omitempty"`
	Status               *int              `json:"status,omitempty"`
	Password             *string           `json:"password,omitempty"`
	PasswordConfirmation *string           `json:"password_confirmation,omitempty"`
	Language             *string           `json:"language,omitempty"`
	MailNotification     *string           `json:"mail_notification,omitempty"`
	NotifiedProjectIDs   *[]int            `json:"notified_project_ids,omitempty"`
	GroupIDs             *[]int            `json:"group_ids,omitempty"`
	MustChangePasswd     *bool             `json:"must_change_passwd,omitempty"`
	GeneratePassword     *bool             `json:"generate_password,omitempty"`
	AuthSourceID         *int              `json:"auth_source_id,omitempty"`
	CustomFieldValues    CustomFieldValues `json:"custom_field_values,omitempty"`
	Pref                 UserPreferences   `json:"-"`
}

// MyAccountUpdate is the writable subset for PUT /my/account.json. The endpoint
// is the only user-write path available to non-admins; we intentionally keep
// the type narrow so admin-only fields cannot be sent here by accident.
// Custom fields are user-writable: Redmine applies only the ones the user may
// edit. Pref is sent as the top-level "pref" object.
type MyAccountUpdate struct {
	FirstName          *string           `json:"firstname,omitempty"`
	LastName           *string           `json:"lastname,omitempty"`
	Mail               *string           `json:"mail,omitempty"`
	Language           *string           `json:"language,omitempty"`
	MailNotification   *string           `json:"mail_notification,omitempty"`
	NotifiedProjectIDs *[]int            `json:"notified_project_ids,omitempty"`
	CustomFieldValues  CustomFieldValues `json:"custom_field_values,omitempty"`
	Pref               UserPreferences   `json:"-"`
}

// UserFilter defines parameters for listing users.
type UserFilter struct {
	// Status is "active", "registered", "locked", "all", or a raw value sent
	// as-is ("*" on Redmine 5.1+). "all" lists every status on all supported
	// versions: it sends an empty status= and, when Redmine 6.0+ rejects that
	// with 422, retries with status=*.
	Status  string
	Name    string // filter by name
	GroupID int
	Sort    string // e.g. "created_on:desc"
	// ExtraParams are raw Redmine filter parameters, applied last.
	ExtraParams FilterParams
	Limit       int
	Offset      int
	// Includes holds include= values; only "auth_source" (Redmine 6.0+, admin
	// only) is rendered by the users index.
	Includes []string
}
