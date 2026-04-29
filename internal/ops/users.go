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
	ID int `json:"id" jsonschema:"Numeric user ID."`
}

type CreateUserInput struct {
	Login     string `json:"login" jsonschema:"Unique login name."`
	Password  string `json:"password" jsonschema:"Initial password for the new account."`
	FirstName string `json:"firstname" jsonschema:"Given name."`
	LastName  string `json:"lastname" jsonschema:"Family name."`
	Mail      string `json:"mail" jsonschema:"Email address."`
	Admin     bool   `json:"admin,omitempty" jsonschema:"Grant admin privileges."`
}

type UpdateUserInput struct {
	ID        int     `json:"id" jsonschema:"Numeric user ID to update."`
	FirstName *string `json:"firstname,omitempty" jsonschema:"New given name."`
	LastName  *string `json:"lastname,omitempty" jsonschema:"New family name."`
	Mail      *string `json:"mail,omitempty" jsonschema:"New email address."`
	Admin     *bool   `json:"admin,omitempty" jsonschema:"Toggle admin privileges."`
	Status    *int    `json:"status,omitempty" jsonschema:"Numeric status code (1 active, 2 registered, 3 locked)."`
}

type DeleteUserInput struct {
	ID int `json:"id" jsonschema:"Numeric user ID to delete. Destructive."`
}

//mcpgen:tool list_users
//mcpgen:description List Redmine users matching the given filter.
//mcpgen:category users
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

//mcpgen:tool get_user
//mcpgen:description Fetch a single Redmine user by numeric ID.
//mcpgen:category users
func GetUser(ctx context.Context, client *api.Client, input GetUserInput) (*models.User, error) {
	return client.Users.Get(ctx, input.ID)
}

//mcpgen:tool me
//mcpgen:description Return the currently authenticated Redmine user.
//mcpgen:category users
func GetCurrentUser(ctx context.Context, client *api.Client, _ struct{}) (*models.User, error) {
	return client.Users.Current(ctx)
}

//mcpgen:tool create_user
//mcpgen:description Create a new Redmine user. Requires --enable-writes and admin privileges.
//mcpgen:category users
//mcpgen:writes
func CreateUser(ctx context.Context, client *api.Client, input CreateUserInput) (*models.User, error) {
	return client.Users.Create(ctx, models.UserCreate{
		Login:     input.Login,
		Password:  input.Password,
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Mail:      input.Mail,
		Admin:     input.Admin,
	})
}

//mcpgen:tool update_user
//mcpgen:description Update an existing Redmine user. Requires --enable-writes.
//mcpgen:category users
//mcpgen:writes
func UpdateUser(ctx context.Context, client *api.Client, input UpdateUserInput) (MessageResult, error) {
	if err := client.Users.Update(ctx, input.ID, models.UserUpdate{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Mail:      input.Mail,
		Admin:     input.Admin,
		Status:    input.Status,
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
