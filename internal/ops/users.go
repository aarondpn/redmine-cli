package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListUsersInput struct {
	Status      string              `json:"status,omitempty" jsonschema:"Filter by status: 'active', 'registered', 'locked', 'all', or a numeric code. Redmine lists only active users when omitted."`
	Name        string              `json:"name,omitempty" jsonschema:"Filter by name substring."`
	GroupID     int                 `json:"group_id,omitempty" jsonschema:"Filter users that belong to the given group ID."`
	Sort        string              `json:"sort,omitempty" jsonschema:"Sort expression, e.g. 'lastname' or 'created_on:desc'."`
	ExtraParams models.FilterParams `json:"extra_params,omitempty" jsonschema:"Raw Redmine filter parameters sent as-is (escape hatch); a key here replaces the same parameter set by another field. Use an array for keys that take several values (f[], v[field][]). With f[], Redmine applies only the filters listed in f[]; filters set by other fields such as status are dropped. The f[]/op[]/v[] filters need Redmine 5.1+; plain keys work on every version. Example: {\"f[]\":[\"status\",\"login\"],\"op[status]\":\"*\",\"op[login]\":\"~\",\"v[login][]\":\"smith\"}."`
	Limit       int                 `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset      int                 `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
	Includes    []string            `json:"includes,omitempty" jsonschema:"Optional includes: 'auth_source' (Redmine 6.0+, admin only)."`
}

type UsersListResult struct {
	Users      []models.User `json:"users"`
	Count      int           `json:"count"`
	TotalCount int           `json:"total_count"`
}

type GetUserInput struct {
	ID       int      `json:"id" jsonschema:"Numeric user ID."`
	Includes []string `json:"includes,omitempty" jsonschema:"Optional includes: 'memberships', 'groups' (Redmine 2.1+), 'auth_source' (Redmine 6.0+, admin only)."`
}

type GetCurrentUserInput struct {
	Includes []string `json:"includes,omitempty" jsonschema:"Optional includes: 'memberships', 'groups' (Redmine 2.1+), 'auth_source' (Redmine 6.0+, admin only)."`
}

type CreateUserInput struct {
	Login             string                   `json:"login" jsonschema:"Unique login name."`
	Password          string                   `json:"password,omitempty" jsonschema:"Initial password. Omit when generate_password is true."`
	FirstName         string                   `json:"firstname" jsonschema:"Given name."`
	LastName          string                   `json:"lastname" jsonschema:"Family name."`
	Mail              string                   `json:"mail" jsonschema:"Email address."`
	Admin             bool                     `json:"admin,omitempty" jsonschema:"Grant admin privileges."`
	Status            *int                     `json:"status,omitempty" jsonschema:"Initial numeric status code (1 active, 2 registered, 3 locked). Redmine creates active users by default."`
	Language          *string                  `json:"language,omitempty" jsonschema:"Interface language code such as 'en', 'de' or 'pt-BR'. Write-only: no API response returns it."`
	MailNotification  *string                  `json:"mail_notification,omitempty" jsonschema:"Email notification preference: 'all', 'selected', 'only_my_events', 'only_assigned', 'only_owner', 'only_my_watches' (Redmine 7.0+), or 'none'."`
	MustChangePasswd  *bool                    `json:"must_change_passwd,omitempty" jsonschema:"Force the user to change their password on next login."`
	GeneratePassword  *bool                    `json:"generate_password,omitempty" jsonschema:"Server generates a random password."`
	AuthSourceID      *int                     `json:"auth_source_id,omitempty" jsonschema:"Numeric authentication source ID for external auth."`
	SendInformation   bool                     `json:"send_information,omitempty" jsonschema:"Email the account info to the new user."`
	CustomFieldValues models.CustomFieldValues `json:"custom_field_values,omitempty" jsonschema:"User custom field values keyed by numeric custom field ID (list_custom_fields, customized_type user). Use a string for one value or an array of strings for a multi-value field; an empty string clears the field."`
	Pref              models.UserPreferences   `json:"pref,omitempty" jsonschema:"User preferences, sent as Redmine's top-level pref object: preference name to string value. Common keys: hide_mail ('1' hides the email address from other users, '0' shows it), time_zone (a Rails time zone name such as 'Berlin' or 'UTC'; '' uses the server default), comments_sorting ('asc' or 'desc'), no_self_notified ('1' skips notifications about the user's own changes, '0' sends them), warn_on_leaving_unsaved ('1' or '0'). Booleans are '1' or '0', not 'true' or 'false'. Newer Redmine versions add keys such as default_issue_query (5.0+); Redmine silently ignores keys it does not know. List-valued preference keys are not supported. Write-only: no API response returns preferences."`
}

// UpdateUserInput is the update_user tool input. Password is set only by the
// CLI (--password-file); it has no JSON name, so the MCP schema does not
// offer it and agents cannot set user passwords.
type UpdateUserInput struct {
	ID                 int                      `json:"id" jsonschema:"Numeric user ID to update."`
	FirstName          *string                  `json:"firstname,omitempty" jsonschema:"New given name."`
	LastName           *string                  `json:"lastname,omitempty" jsonschema:"New family name."`
	Mail               *string                  `json:"mail,omitempty" jsonschema:"New email address."`
	Admin              *bool                    `json:"admin,omitempty" jsonschema:"Toggle admin privileges."`
	Status             *int                     `json:"status,omitempty" jsonschema:"Numeric status code (1 active, 2 registered, 3 locked)."`
	Language           *string                  `json:"language,omitempty" jsonschema:"Interface language code such as 'en', 'de' or 'pt-BR'; an empty string resets it to the server default. Write-only: no API response returns it."`
	MailNotification   *string                  `json:"mail_notification,omitempty" jsonschema:"Email notification preference: 'all', 'selected', 'only_my_events', 'only_assigned', 'only_owner', 'only_my_watches' (Redmine 7.0+), or 'none'. 'selected' notifies about the projects in notified_project_ids."`
	NotifiedProjectIDs *[]int                   `json:"notified_project_ids,omitempty" jsonschema:"Numeric IDs of the projects to get all notifications for when mail_notification is 'selected'. Only projects the user is a member of count. Replaces the current set; an empty list clears it."`
	GroupIDs           *[]int                   `json:"group_ids,omitempty" jsonschema:"Numeric group IDs the user belongs to (list_groups). Replaces the user's current groups; an empty list removes the user from all groups."`
	MustChangePasswd   *bool                    `json:"must_change_passwd,omitempty" jsonschema:"Force the user to change their password on next login."`
	GeneratePassword   *bool                    `json:"generate_password,omitempty" jsonschema:"Server generates a random password."`
	AuthSourceID       *int                     `json:"auth_source_id,omitempty" jsonschema:"Numeric authentication source ID for external auth."`
	CustomFieldValues  models.CustomFieldValues `json:"custom_field_values,omitempty" jsonschema:"User custom field values keyed by numeric custom field ID (list_custom_fields, customized_type user). Use a string for one value or an array of strings for a multi-value field; an empty string clears the field."`
	Pref               models.UserPreferences   `json:"pref,omitempty" jsonschema:"User preferences, sent as Redmine's top-level pref object: preference name to string value. Common keys: hide_mail ('1' hides the email address from other users, '0' shows it), time_zone (a Rails time zone name such as 'Berlin' or 'UTC'; '' uses the server default), comments_sorting ('asc' or 'desc'), no_self_notified ('1' skips notifications about the user's own changes, '0' sends them), warn_on_leaving_unsaved ('1' or '0'). Booleans are '1' or '0', not 'true' or 'false'. Newer Redmine versions add keys such as default_issue_query (5.0+); Redmine silently ignores keys it does not know. List-valued preference keys are not supported. Write-only: no API response returns preferences."`

	// Password is the new password (CLI only, never part of the MCP schema).
	// It is sent together with an identical password_confirmation.
	Password string `json:"-"`
}

type DeleteUserInput struct {
	ID int `json:"id" jsonschema:"Numeric user ID to delete. Destructive."`
}

//mcpgen:tool list_users
//mcpgen:description List Redmine users matching the given filter. Supports the auth_source include.
//mcpgen:category users
func ListUsers(ctx context.Context, client *api.Client, input ListUsersInput) (UsersListResult, error) {
	users, total, err := client.Users.List(ctx, models.UserFilter{
		Status:      input.Status,
		Name:        input.Name,
		GroupID:     input.GroupID,
		Sort:        input.Sort,
		ExtraParams: input.ExtraParams,
		Limit:       ListLimit(input.Limit),
		Offset:      input.Offset,
		Includes:    input.Includes,
	})
	if err != nil {
		return UsersListResult{}, err
	}
	return UsersListResult{Users: users, Count: len(users), TotalCount: total}, nil
}

//mcpgen:tool get_user
//mcpgen:description Fetch a single Redmine user by numeric ID. Supports memberships, groups and auth_source includes.
//mcpgen:category users
func GetUser(ctx context.Context, client *api.Client, input GetUserInput) (*models.User, error) {
	return client.Users.Get(ctx, input.ID, input.Includes)
}

//mcpgen:tool me
//mcpgen:description Return the currently authenticated Redmine user. Supports memberships, groups and auth_source includes.
//mcpgen:category users
func GetCurrentUser(ctx context.Context, client *api.Client, input GetCurrentUserInput) (*models.User, error) {
	return client.Users.Current(ctx, input.Includes)
}

//mcpgen:tool create_user
//mcpgen:description Create a new Redmine user. Requires --enable-writes and admin privileges.
//mcpgen:category users
//mcpgen:writes
func CreateUser(ctx context.Context, client *api.Client, input CreateUserInput) (*models.User, error) {
	return client.Users.Create(ctx, models.UserCreate{
		Login:             input.Login,
		Password:          input.Password,
		FirstName:         input.FirstName,
		LastName:          input.LastName,
		Mail:              input.Mail,
		Admin:             input.Admin,
		Status:            input.Status,
		Language:          input.Language,
		MailNotification:  input.MailNotification,
		MustChangePasswd:  input.MustChangePasswd,
		GeneratePassword:  input.GeneratePassword,
		AuthSourceID:      input.AuthSourceID,
		CustomFieldValues: input.CustomFieldValues,
		Pref:              input.Pref,
	}, input.SendInformation)
}

//mcpgen:tool update_user
//mcpgen:description Update an existing Redmine user. Passwords cannot be set through this tool. Requires --enable-writes.
//mcpgen:category users
//mcpgen:writes
func UpdateUser(ctx context.Context, client *api.Client, input UpdateUserInput) (MessageResult, error) {
	update := models.UserUpdate{
		FirstName:          input.FirstName,
		LastName:           input.LastName,
		Mail:               input.Mail,
		Admin:              input.Admin,
		Status:             input.Status,
		Language:           input.Language,
		MailNotification:   input.MailNotification,
		NotifiedProjectIDs: input.NotifiedProjectIDs,
		GroupIDs:           input.GroupIDs,
		MustChangePasswd:   input.MustChangePasswd,
		GeneratePassword:   input.GeneratePassword,
		AuthSourceID:       input.AuthSourceID,
		CustomFieldValues:  input.CustomFieldValues,
		Pref:               input.Pref,
	}
	if input.Password != "" {
		// Redmine checks the confirmation against the password; the CLI has
		// already confirmed it, so both carry the same value.
		update.Password = &input.Password
		update.PasswordConfirmation = &input.Password
	}
	if err := client.Users.Update(ctx, input.ID, update); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated user %d", input.ID)}, nil
}

//mcpgen:tool delete_user
//mcpgen:description Delete a Redmine user. Destructive. Requires --enable-writes.
//mcpgen:category users
//mcpgen:writes
func DeleteUser(ctx context.Context, client *api.Client, input DeleteUserInput) (MessageResult, error) {
	if err := client.Users.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted user %d", input.ID)}, nil
}
