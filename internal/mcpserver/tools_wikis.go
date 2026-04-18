package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type listWikiPagesArgs struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type wikiPagesListResult struct {
	Pages      []models.WikiPageIndex `json:"pages"`
	Count      int                    `json:"count"`
	TotalCount int                    `json:"total_count"`
}

type getWikiPageArgs struct {
	ProjectID string   `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string   `json:"page" jsonschema:"Wiki page title (slug)."`
	Includes  []string `json:"includes,omitempty" jsonschema:"Extra sections to include, e.g. 'attachments'."`
}

type createWikiPageArgs struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string `json:"page" jsonschema:"Wiki page title (slug) to create or overwrite."`
	Text      string `json:"text" jsonschema:"Page body (Textile or Markdown depending on the Redmine configuration)."`
	Title     string `json:"title,omitempty" jsonschema:"Optional display title; may differ from the slug."`
	Comments  string `json:"comments,omitempty" jsonschema:"Edit comment."`
}

type updateWikiPageArgs struct {
	ProjectID string  `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string  `json:"page" jsonschema:"Wiki page title (slug) to update."`
	Text      *string `json:"text,omitempty" jsonschema:"New page body."`
	Title     *string `json:"title,omitempty" jsonschema:"New display title."`
	Comments  *string `json:"comments,omitempty" jsonschema:"Edit comment."`
}

type deleteWikiPageArgs struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string `json:"page" jsonschema:"Wiki page title (slug) to delete. Destructive."`
}

func registerWikiTools(s *mcp.Server, client *api.Client, opts Options) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_wiki_pages",
		Description: "List wiki pages for a project.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listWikiPagesArgs) (*mcp.CallToolResult, any, error) {
		pages, total, err := client.Wikis.List(ctx, args.ProjectID, listLimit(args.Limit), args.Offset)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](wikiPagesListResult{Pages: pages, Count: len(pages), TotalCount: total})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_wiki_page",
		Description: "Fetch a single wiki page.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getWikiPageArgs) (*mcp.CallToolResult, any, error) {
		page, err := client.Wikis.Get(ctx, args.ProjectID, args.Page, args.Includes)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](page)
	})

	if !opts.EnableWrites {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_wiki_page",
		Description: "Create (or overwrite) a wiki page. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createWikiPageArgs) (*mcp.CallToolResult, any, error) {
		page, err := client.Wikis.Create(ctx, args.ProjectID, args.Page, models.WikiPageCreate{
			Text:     args.Text,
			Title:    args.Title,
			Comments: args.Comments,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](page)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_wiki_page",
		Description: "Update an existing wiki page. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateWikiPageArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Wikis.Update(ctx, args.ProjectID, args.Page, models.WikiPageUpdate{
			Text:     args.Text,
			Title:    args.Title,
			Comments: args.Comments,
		}); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Updated wiki page %s/%s", args.ProjectID, args.Page))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_wiki_page",
		Description: "Delete a wiki page. Destructive. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteWikiPageArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Wikis.Delete(ctx, args.ProjectID, args.Page); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Deleted wiki page %s/%s", args.ProjectID, args.Page))
	})
}
