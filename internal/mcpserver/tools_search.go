package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type searchArgs struct {
	Query      string `json:"query" jsonschema:"Full-text search query."`
	ProjectID  string `json:"project_id,omitempty" jsonschema:"Scope search to a single project (identifier or numeric ID)."`
	Scope      string `json:"scope,omitempty" jsonschema:"One of 'all', 'my_projects', 'subprojects'."`
	Issues     bool   `json:"issues,omitempty" jsonschema:"Include issues in results."`
	News       bool   `json:"news,omitempty" jsonschema:"Include news in results."`
	Documents  bool   `json:"documents,omitempty" jsonschema:"Include documents in results."`
	Changesets bool   `json:"changesets,omitempty" jsonschema:"Include changesets in results."`
	WikiPages  bool   `json:"wiki_pages,omitempty" jsonschema:"Include wiki pages in results."`
	Messages   bool   `json:"messages,omitempty" jsonschema:"Include forum messages in results."`
	Projects   bool   `json:"projects,omitempty" jsonschema:"Include projects in results."`
	AllWords   bool   `json:"all_words,omitempty" jsonschema:"Require all query words to match."`
	TitlesOnly bool   `json:"titles_only,omitempty" jsonschema:"Match query against titles only."`
	OpenIssues bool   `json:"open_issues,omitempty" jsonschema:"Limit issue results to open issues."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type searchListResult struct {
	Results    []models.SearchResult `json:"results"`
	Count      int                   `json:"count"`
	TotalCount int                   `json:"total_count"`
}

func registerSearchTools(s *mcp.Server, client *api.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search",
		Description: "Search across Redmine issues, wiki pages, news, and more. If no type flag is set, issues and wiki pages are included by default.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, any, error) {
		anySelected := args.Issues || args.News || args.Documents ||
			args.Changesets || args.WikiPages || args.Messages || args.Projects
		params := api.SearchParams{
			Query:      args.Query,
			ProjectID:  args.ProjectID,
			Scope:      args.Scope,
			AllWords:   args.AllWords,
			TitlesOnly: args.TitlesOnly,
			OpenIssues: args.OpenIssues,
			Issues:     args.Issues,
			News:       args.News,
			Documents:  args.Documents,
			Changesets: args.Changesets,
			WikiPages:  args.WikiPages,
			Messages:   args.Messages,
			Projects:   args.Projects,
			Limit:      listLimit(args.Limit),
			Offset:     args.Offset,
		}
		if !anySelected {
			params.Issues = true
			params.WikiPages = true
		}
		results, total, err := client.Search.Search(ctx, params)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](searchListResult{Results: results, Count: len(results), TotalCount: total})
	})
}
