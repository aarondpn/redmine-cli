package mcpserver

import (
	"context"
	"fmt"

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

type createVersionArgs struct {
	ProjectID     string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Name          string `json:"name" jsonschema:"Version name."`
	Status        string `json:"status,omitempty" jsonschema:"Version status: open, locked, or closed."`
	Sharing       string `json:"sharing,omitempty" jsonschema:"Version sharing: none, descendants, hierarchy, tree, or system."`
	DueDate       string `json:"due_date,omitempty" jsonschema:"Due date (YYYY-MM-DD)."`
	Description   string `json:"description,omitempty" jsonschema:"Version description."`
	WikiPageTitle string `json:"wiki_page_title,omitempty" jsonschema:"Associated wiki page title."`
}

type updateVersionArgs struct {
	ID            int     `json:"id" jsonschema:"Numeric version (milestone) ID."`
	Name          *string `json:"name,omitempty" jsonschema:"New version name."`
	Status        *string `json:"status,omitempty" jsonschema:"New status: open, locked, or closed."`
	Sharing       *string `json:"sharing,omitempty" jsonschema:"New sharing: none, descendants, hierarchy, tree, or system."`
	DueDate       *string `json:"due_date,omitempty" jsonschema:"New due date (YYYY-MM-DD)."`
	Description   *string `json:"description,omitempty" jsonschema:"New description."`
	WikiPageTitle *string `json:"wiki_page_title,omitempty" jsonschema:"New associated wiki page title."`
}

type deleteVersionArgs struct {
	ID int `json:"id" jsonschema:"Numeric version (milestone) ID to delete. Destructive."`
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

func registerMetaTools(s *mcp.Server, client *api.Client, opts Options) {
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

	if opts.EnableWrites {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "create_version",
			Description: "Create a project version (milestone). Requires --enable-writes.",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args createVersionArgs) (*mcp.CallToolResult, any, error) {
			version, err := client.Versions.Create(ctx, args.ProjectID, models.VersionCreate{
				Name:          args.Name,
				Status:        args.Status,
				Sharing:       args.Sharing,
				DueDate:       args.DueDate,
				Description:   args.Description,
				WikiPageTitle: args.WikiPageTitle,
			})
			if err != nil {
				return toolErrFromAPI[any](err)
			}
			return toolOK[any](version)
		})

		mcp.AddTool(s, &mcp.Tool{
			Name:        "update_version",
			Description: "Update an existing version (milestone). Requires --enable-writes.",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateVersionArgs) (*mcp.CallToolResult, any, error) {
			if err := client.Versions.Update(ctx, args.ID, models.VersionUpdate{
				Name:          args.Name,
				Status:        args.Status,
				Sharing:       args.Sharing,
				DueDate:       args.DueDate,
				Description:   args.Description,
				WikiPageTitle: args.WikiPageTitle,
			}); err != nil {
				return toolErrFromAPI[any](err)
			}
			return toolOKMsg(fmt.Sprintf("Updated version %d", args.ID))
		})

		mcp.AddTool(s, &mcp.Tool{
			Name:        "delete_version",
			Description: "Delete a version (milestone). Destructive. Requires --enable-writes.",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteVersionArgs) (*mcp.CallToolResult, any, error) {
			if err := client.Versions.Delete(ctx, args.ID); err != nil {
				return toolErrFromAPI[any](err)
			}
			return toolOKMsg(fmt.Sprintf("Deleted version %d", args.ID))
		})
	}

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
