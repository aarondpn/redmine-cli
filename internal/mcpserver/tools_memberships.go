package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type listMembershipsArgs struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type membershipsListResult struct {
	Memberships []models.Membership `json:"memberships"`
	Count       int                 `json:"count"`
	TotalCount  int                 `json:"total_count"`
}

type getMembershipArgs struct {
	ID int `json:"id" jsonschema:"Numeric membership ID."`
}

type createMembershipArgs struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	UserID    int    `json:"user_id" jsonschema:"Numeric user ID to add as a member."`
	RoleIDs   []int  `json:"role_ids" jsonschema:"One or more role IDs to grant."`
}

type updateMembershipArgs struct {
	ID      int   `json:"id" jsonschema:"Numeric membership ID."`
	RoleIDs []int `json:"role_ids" jsonschema:"Replacement set of role IDs."`
}

type deleteMembershipArgs struct {
	ID int `json:"id" jsonschema:"Numeric membership ID to delete. Destructive."`
}

func registerMembershipTools(s *mcp.Server, client *api.Client, opts Options) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_memberships",
		Description: "List memberships for a project.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listMembershipsArgs) (*mcp.CallToolResult, any, error) {
		members, total, err := client.Memberships.List(ctx, args.ProjectID, listLimit(args.Limit), args.Offset)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](membershipsListResult{Memberships: members, Count: len(members), TotalCount: total})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_membership",
		Description: "Fetch a single project membership by ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getMembershipArgs) (*mcp.CallToolResult, any, error) {
		m, err := client.Memberships.Get(ctx, args.ID)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](m)
	})

	if !opts.EnableWrites {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_membership",
		Description: "Add a user to a project with the given roles. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createMembershipArgs) (*mcp.CallToolResult, any, error) {
		m, err := client.Memberships.Create(ctx, args.ProjectID, models.MembershipCreate{
			UserID:  args.UserID,
			RoleIDs: args.RoleIDs,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](m)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_membership",
		Description: "Replace the roles on a project membership. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateMembershipArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Memberships.Update(ctx, args.ID, models.MembershipUpdate{RoleIDs: args.RoleIDs}); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Updated membership %d", args.ID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_membership",
		Description: "Remove a project membership. Destructive. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteMembershipArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Memberships.Delete(ctx, args.ID); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Deleted membership %d", args.ID))
	})
}
