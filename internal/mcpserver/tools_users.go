package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type listUsersArgs struct {
	Status  string `json:"status,omitempty" jsonschema:"Filter by status: 'active', 'registered', 'locked', or a numeric code."`
	Name    string `json:"name,omitempty" jsonschema:"Filter by name substring."`
	GroupID int    `json:"group_id,omitempty" jsonschema:"Filter users that belong to the given group ID."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type usersListResult struct {
	Users      []models.User `json:"users"`
	Count      int           `json:"count"`
	TotalCount int           `json:"total_count"`
}

type getUserArgs struct {
	ID int `json:"id" jsonschema:"Numeric user ID."`
}

type createUserArgs struct {
	Login     string `json:"login" jsonschema:"Unique login name."`
	Password  string `json:"password" jsonschema:"Initial password for the new account."`
	FirstName string `json:"firstname" jsonschema:"Given name."`
	LastName  string `json:"lastname" jsonschema:"Family name."`
	Mail      string `json:"mail" jsonschema:"Email address."`
	Admin     bool   `json:"admin,omitempty" jsonschema:"Grant admin privileges."`
}

type updateUserArgs struct {
	ID        int     `json:"id" jsonschema:"Numeric user ID to update."`
	FirstName *string `json:"firstname,omitempty" jsonschema:"New given name."`
	LastName  *string `json:"lastname,omitempty" jsonschema:"New family name."`
	Mail      *string `json:"mail,omitempty" jsonschema:"New email address."`
	Admin     *bool   `json:"admin,omitempty" jsonschema:"Toggle admin privileges."`
	Status    *int    `json:"status,omitempty" jsonschema:"Numeric status code (1 active, 2 registered, 3 locked)."`
}

type deleteUserArgs struct {
	ID int `json:"id" jsonschema:"Numeric user ID to delete. Destructive."`
}

func registerUserTools(s *mcp.Server, client *api.Client, opts Options) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_users",
		Description: "List Redmine users matching the given filter.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listUsersArgs) (*mcp.CallToolResult, any, error) {
		users, total, err := client.Users.List(ctx, models.UserFilter{
			Status:  args.Status,
			Name:    args.Name,
			GroupID: args.GroupID,
			Limit:   listLimit(args.Limit),
			Offset:  args.Offset,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](usersListResult{Users: users, Count: len(users), TotalCount: total})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_user",
		Description: "Fetch a single Redmine user by numeric ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getUserArgs) (*mcp.CallToolResult, any, error) {
		user, err := client.Users.Get(ctx, args.ID)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](user)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "me",
		Description: "Return the currently authenticated Redmine user.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		user, err := client.Users.Current(ctx)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](user)
	})

	if !opts.EnableWrites {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_user",
		Description: "Create a new Redmine user. Requires --enable-writes and admin privileges.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createUserArgs) (*mcp.CallToolResult, any, error) {
		user, err := client.Users.Create(ctx, models.UserCreate{
			Login:     args.Login,
			Password:  args.Password,
			FirstName: args.FirstName,
			LastName:  args.LastName,
			Mail:      args.Mail,
			Admin:     args.Admin,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](user)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_user",
		Description: "Update an existing Redmine user. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateUserArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Users.Update(ctx, args.ID, models.UserUpdate{
			FirstName: args.FirstName,
			LastName:  args.LastName,
			Mail:      args.Mail,
			Admin:     args.Admin,
			Status:    args.Status,
		}); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Updated user %d", args.ID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_user",
		Description: "Delete a Redmine user. Destructive. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteUserArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Users.Delete(ctx, args.ID); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Deleted user %d", args.ID))
	})
}
