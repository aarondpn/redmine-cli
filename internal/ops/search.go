package ops

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type SearchInput struct {
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

type SearchResultsListResult struct {
	Results    []models.SearchResult `json:"results"`
	Count      int                   `json:"count"`
	TotalCount int                   `json:"total_count"`
}

//mcpgen:tool search
//mcpgen:description Search across Redmine issues, wiki pages, news, and more. If no type flag is set, issues and wiki pages are included by default.
//mcpgen:category search
func Search(ctx context.Context, client *api.Client, input SearchInput) (SearchResultsListResult, error) {
	anySelected := input.Issues || input.News || input.Documents ||
		input.Changesets || input.WikiPages || input.Messages || input.Projects
	params := api.SearchParams{
		Query:      input.Query,
		ProjectID:  input.ProjectID,
		Scope:      input.Scope,
		AllWords:   input.AllWords,
		TitlesOnly: input.TitlesOnly,
		OpenIssues: input.OpenIssues,
		Issues:     input.Issues,
		News:       input.News,
		Documents:  input.Documents,
		Changesets: input.Changesets,
		WikiPages:  input.WikiPages,
		Messages:   input.Messages,
		Projects:   input.Projects,
		Limit:      ListLimit(input.Limit),
		Offset:     input.Offset,
	}
	if !anySelected {
		params.Issues = true
		params.WikiPages = true
	}
	results, total, err := client.Search.Search(ctx, params)
	if err != nil {
		return SearchResultsListResult{}, err
	}
	return SearchResultsListResult{Results: results, Count: len(results), TotalCount: total}, nil
}
