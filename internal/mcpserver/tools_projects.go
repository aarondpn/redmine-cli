package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type listProjectsArgs struct {
	Includes []string `json:"includes,omitempty" jsonschema:"Extra sections to include: trackers, issue_categories, enabled_modules."`
	Limit    int      `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset   int      `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type projectsListResult struct {
	Projects   []models.Project `json:"projects"`
	Count      int              `json:"count"`
	TotalCount int              `json:"total_count"`
}

type getProjectArgs struct {
	Identifier string   `json:"identifier" jsonschema:"Project identifier (slug) or numeric ID."`
	Includes   []string `json:"includes,omitempty" jsonschema:"Extra sections to include: trackers, issue_categories, enabled_modules."`
}

type createProjectArgs struct {
	Name           string `json:"name" jsonschema:"Human-readable project name."`
	Identifier     string `json:"identifier" jsonschema:"URL-safe project identifier (slug)."`
	Description    string `json:"description,omitempty" jsonschema:"Project description."`
	IsPublic       *bool  `json:"is_public,omitempty" jsonschema:"Mark the project as public."`
	ParentID       int    `json:"parent_id,omitempty" jsonschema:"Parent project numeric ID."`
	InheritMembers bool   `json:"inherit_members,omitempty" jsonschema:"Inherit members from the parent project."`
}

type updateProjectArgs struct {
	Identifier  string  `json:"identifier" jsonschema:"Project identifier or numeric ID to update."`
	Name        *string `json:"name,omitempty" jsonschema:"New project name."`
	Description *string `json:"description,omitempty" jsonschema:"New project description."`
	IsPublic    *bool   `json:"is_public,omitempty" jsonschema:"Toggle public visibility."`
}

type deleteProjectArgs struct {
	Identifier string `json:"identifier" jsonschema:"Project identifier or numeric ID to delete. Destructive."`
}

type listProjectMembersArgs struct {
	Identifier string `json:"identifier" jsonschema:"Project identifier or numeric ID."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type membersListResult struct {
	Members    []models.Membership `json:"members"`
	Count      int                 `json:"count"`
	TotalCount int                 `json:"total_count"`
}

func registerProjectTools(s *mcp.Server, client *api.Client, opts Options) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_projects",
		Description: "List Redmine projects visible to the authenticated user.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listProjectsArgs) (*mcp.CallToolResult, any, error) {
		projects, total, err := client.Projects.List(ctx, args.Includes, listLimit(args.Limit), args.Offset)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](projectsListResult{Projects: projects, Count: len(projects), TotalCount: total})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_project",
		Description: "Fetch a single Redmine project by identifier or numeric ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getProjectArgs) (*mcp.CallToolResult, any, error) {
		project, err := client.Projects.Get(ctx, args.Identifier, args.Includes)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](project)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_project_members",
		Description: "List memberships of a project (users and groups with roles).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listProjectMembersArgs) (*mcp.CallToolResult, any, error) {
		members, total, err := client.Projects.Members(ctx, args.Identifier, listLimit(args.Limit), args.Offset)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](membersListResult{Members: members, Count: len(members), TotalCount: total})
	})

	if !opts.EnableWrites {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_project",
		Description: "Create a new Redmine project. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createProjectArgs) (*mcp.CallToolResult, any, error) {
		project, err := client.Projects.Create(ctx, models.ProjectCreate{
			Name:           args.Name,
			Identifier:     args.Identifier,
			Description:    args.Description,
			IsPublic:       args.IsPublic,
			ParentID:       args.ParentID,
			InheritMembers: args.InheritMembers,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](project)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_project",
		Description: "Update an existing Redmine project. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateProjectArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Projects.Update(ctx, args.Identifier, models.ProjectUpdate{
			Name:        args.Name,
			Description: args.Description,
			IsPublic:    args.IsPublic,
		}); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Updated project %s", args.Identifier))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_project",
		Description: "Delete a Redmine project. Destructive. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteProjectArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Projects.Delete(ctx, args.Identifier); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Deleted project %s", args.Identifier))
	})
}
