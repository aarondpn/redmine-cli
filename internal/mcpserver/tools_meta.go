package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type listVersionsArgs struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type versionsListResult struct {
	Versions   []models.Version `json:"versions"`
	Count      int              `json:"count"`
	TotalCount int              `json:"total_count"`
}

type getVersionArgs struct {
	ID int `json:"id" jsonschema:"Numeric version (milestone) ID."`
}

type listCategoriesArgs struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
}

type categoriesListResult struct {
	Categories []models.IssueCategory `json:"issue_categories"`
	Count      int                    `json:"count"`
	TotalCount int                    `json:"total_count"`
}

type trackersListResult struct {
	Trackers []models.Tracker `json:"trackers"`
	Count    int              `json:"count"`
}

type statusesListResult struct {
	Statuses []models.IssueStatus `json:"issue_statuses"`
	Count    int                  `json:"count"`
}

func registerMetaTools(s *mcp.Server, client *api.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_versions",
		Description: "List versions (milestones) for a project.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listVersionsArgs) (*mcp.CallToolResult, any, error) {
		versions, total, err := client.Versions.List(ctx, args.ProjectID, listLimit(args.Limit), args.Offset)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](versionsListResult{Versions: versions, Count: len(versions), TotalCount: total})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_version",
		Description: "Fetch a single version (milestone) by ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getVersionArgs) (*mcp.CallToolResult, any, error) {
		version, err := client.Versions.Get(ctx, args.ID)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](version)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_trackers",
		Description: "List all trackers (Bug, Feature, ...) configured in this Redmine instance.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		trackers, err := client.Trackers.List(ctx)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](trackersListResult{Trackers: trackers, Count: len(trackers)})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_statuses",
		Description: "List all issue statuses configured in this Redmine instance.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		statuses, err := client.Statuses.List(ctx)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](statusesListResult{Statuses: statuses, Count: len(statuses)})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_categories",
		Description: "List issue categories for a project.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listCategoriesArgs) (*mcp.CallToolResult, any, error) {
		cats, total, err := client.Categories.List(ctx, args.ProjectID)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](categoriesListResult{Categories: cats, Count: len(cats), TotalCount: total})
	})
}
