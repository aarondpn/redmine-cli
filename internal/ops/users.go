package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListUsersInput struct {
	Status  string `json:"status,omitempty" jsonschema:"Filter by status: 'active', 'registered', 'locked', or a numeric code."`
	Name    string `json:"name,omitempty" jsonschema:"Filter by name substring."`
	GroupID int    `json:"group_id,omitempty" jsonschema:"Filter users that belong to the given group ID."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type UsersListResult struct {
	Users      []models.User `json:"users"`
	Count      int           `json:"count"`
	TotalCount int           `json:"total_count"`
}

type GetUserInput struct {
	ID       int      `json:"id" jsonschema:"Numeric user ID."`
	Includes []string `json:"includes,omitempty" jsonschema:"Optional includes: 'memberships', 'groups' (Redmine 2.1+)."`
}

type GetCurrentUserInput struct {
	Includes []string `json:"includes,omitempty" jsonschema:"Optional includes: 'memberships', 'groups' (Redmine 2.1+)."`
}

type CreateUserInput struct {
	Login            string  `json:"login" jsonschema:"Unique login name."`
	Password         string  `json:"password,omitempty" jsonschema:"Initial password. Omit when generate_password is true."`
	FirstName        string  `json:"firstname" jsonschema:"Given name."`
	LastName         string  `json:"lastname" jsonschema:"Family name."`
	Mail             string  `json:"mail" jsonschema:"Email address."`
	Admin            bool    `json:"admin,omitempty" jsonschema:"Grant admin privileges."`
	MailNotification *string `json:"mail_notification,omitempty" jsonschema:"Email notification preference: 'all', 'only_my_events', 'only_assigned', 'only_owner', or 'none'."`
	MustChangePasswd *bool   `json:"must_change_passwd,omitempty" jsonschema:"Force the user to change their password on next login."`
	GeneratePassword *bool   `json:"generate_password,omitempty" jsonschema:"Server generates a random password."`
	AuthSourceID     *int    `json:"auth_source_id,omitempty" jsonschema:"Numeric authentication source ID for external auth."`
	SendInformation  bool    `json:"send_information,omitempty" jsonschema:"Email the account info to the new user."`
}

type UpdateUserInput struct {
	ID               int     `json:"id" jsonschema:"Numeric user ID to update."`
	FirstName        *string `json:"firstname,omitempty" jsonschema:"New given name."`
	LastName         *string `json:"lastname,omitempty" jsonschema:"New family name."`
	Mail             *string `json:"mail,omitempty" jsonschema:"New email address."`
	Admin            *bool   `json:"admin,omitempty" jsonschema:"Toggle admin privileges."`
	Status           *int    `json:"status,omitempty" jsonschema:"Numeric status code (1 active, 2 registered, 3 locked)."`
	MailNotification *string `json:"mail_notification,omitempty" jsonschema:"Email notification preference: 'all', 'only_my_events', 'only_assigned', 'only_owner', or 'none'."`
	MustChangePasswd *bool   `json:"must_change_passwd,omitempty" jsonschema:"Force the user to change their password on next login."`
	GeneratePassword *bool   `json:"generate_password,omitempty" jsonschema:"Server generates a random password."`
	AuthSourceID     *int    `json:"auth_source_id,omitempty" jsonschema:"Numeric authentication source ID for external auth."`
}

type DeleteUserInput struct {
	ID int `json:"id" jsonschema:"Numeric user ID to delete. Destructive."`
}

// ListUsers lists users matching the filter. The CLI calls it directly; the
// list_users MCP tool goes through ListUsersRedacted.
func ListUsers(ctx context.Context, client *api.Client, input ListUsersInput) (UsersListResult, error) {
	users, total, err := client.Users.List(ctx, models.UserFilter{
		Status:  input.Status,
		Name:    input.Name,
		GroupID: input.GroupID,
		Limit:   ListLimit(input.Limit),
		Offset:  input.Offset,
	})
	if err != nil {
		return UsersListResult{}, err
	}
	return UsersListResult{Users: users, Count: len(users), TotalCount: total}, nil
}

// GetUser fetches a single user. The CLI calls it directly; the get_user MCP
// tool goes through GetUserRedacted.
func GetUser(ctx context.Context, client *api.Client, input GetUserInput) (*models.User, error) {
	return client.Users.Get(ctx, input.ID, input.Includes)
}

// GetCurrentUser fetches the authenticated user. The CLI calls it directly;
// the me MCP tool goes through GetCurrentUserRedacted.
func GetCurrentUser(ctx context.Context, client *api.Client, input GetCurrentUserInput) (*models.User, error) {
	return client.Users.Current(ctx, input.Includes)
}

// RedactUser clears credential fields (the API key) from u in place and
// returns it. Every MCP-facing user operation calls it: neither the key the
// server runs with nor another user's key returned to an admin caller may
// reach an MCP client. A nil u is returned unchanged.
func RedactUser(u *models.User) *models.User {
	if u != nil {
		u.APIKey = ""
	}
	return u
}

//mcpgen:tool list_users
//mcpgen:description List Redmine users matching the given filter.
//mcpgen:category users
func ListUsersRedacted(ctx context.Context, client *api.Client, input ListUsersInput) (UsersListResult, error) {
	result, err := ListUsers(ctx, client, input)
	for i := range result.Users {
		RedactUser(&result.Users[i])
	}
	return result, err
}

//mcpgen:tool get_user
//mcpgen:description Fetch a single Redmine user by numeric ID. Supports memberships and groups includes.
//mcpgen:category users
func GetUserRedacted(ctx context.Context, client *api.Client, input GetUserInput) (*models.User, error) {
	user, err := GetUser(ctx, client, input)
	return RedactUser(user), err
}

//mcpgen:tool me
//mcpgen:description Return the currently authenticated Redmine user. Supports memberships and groups includes.
//mcpgen:category users
func GetCurrentUserRedacted(ctx context.Context, client *api.Client, input GetCurrentUserInput) (*models.User, error) {
	user, err := GetCurrentUser(ctx, client, input)
	return RedactUser(user), err
}

//mcpgen:tool create_user
//mcpgen:description Create a new Redmine user. Requires --enable-writes and admin privileges.
//mcpgen:category users
//mcpgen:writes
func CreateUserRedacted(ctx context.Context, client *api.Client, input CreateUserInput) (*models.User, error) {
	user, err := CreateUser(ctx, client, input)
	return RedactUser(user), err
}

// CreateUser creates a user. The CLI calls it directly; the create_user MCP
// tool goes through CreateUserRedacted.
func CreateUser(ctx context.Context, client *api.Client, input CreateUserInput) (*models.User, error) {
	return client.Users.Create(ctx, models.UserCreate{
		Login:            input.Login,
		Password:         input.Password,
		FirstName:        input.FirstName,
		LastName:         input.LastName,
		Mail:             input.Mail,
		Admin:            input.Admin,
		MailNotification: input.MailNotification,
		MustChangePasswd: input.MustChangePasswd,
		GeneratePassword: input.GeneratePassword,
		AuthSourceID:     input.AuthSourceID,
	}, input.SendInformation)
}

//mcpgen:tool update_user
//mcpgen:description Update an existing Redmine user. Requires --enable-writes.
//mcpgen:category users
//mcpgen:writes
func UpdateUser(ctx context.Context, client *api.Client, input UpdateUserInput) (MessageResult, error) {
	if err := client.Users.Update(ctx, input.ID, models.UserUpdate{
		FirstName:        input.FirstName,
		LastName:         input.LastName,
		Mail:             input.Mail,
		Admin:            input.Admin,
		Status:           input.Status,
		MailNotification: input.MailNotification,
		MustChangePasswd: input.MustChangePasswd,
		GeneratePassword: input.GeneratePassword,
		AuthSourceID:     input.AuthSourceID,
	}); err != nil {
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
