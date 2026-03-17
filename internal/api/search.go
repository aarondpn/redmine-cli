package api

import (
	"context"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// SearchService handles the Redmine search API.
type SearchService struct {
	client *Client
}

// SearchParams holds parameters for the search API call.
type SearchParams struct {
	Query       string
	ProjectID   string
	Scope       string // "all", "my_projects", "subprojects"
	AllWords    bool
	TitlesOnly  bool
	OpenIssues  bool
	Attachments string // "0", "1", "only"

	// Resource type filters
	Issues     bool
	News       bool
	Documents  bool
	Changesets bool
	WikiPages  bool
	Messages   bool
	Projects   bool

	Limit  int
	Offset int
}

// Search performs a search query against the Redmine search API.
func (s *SearchService) Search(ctx context.Context, params SearchParams) ([]models.SearchResult, int, error) {
	vals := url.Values{}
	vals.Set("q", params.Query)

	if params.ProjectID != "" {
		// Project-scoped search uses /projects/:id/search.json
	}
	if params.Scope != "" {
		vals.Set("scope", params.Scope)
	}
	if params.AllWords {
		vals.Set("all_words", "1")
	}
	if params.TitlesOnly {
		vals.Set("titles_only", "1")
	}
	if params.OpenIssues {
		vals.Set("open_issues", "1")
	}
	if params.Attachments != "" {
		vals.Set("attachments", params.Attachments)
	}

	// Resource type filters
	if params.Issues {
		vals.Set("issues", "1")
	}
	if params.News {
		vals.Set("news", "1")
	}
	if params.Documents {
		vals.Set("documents", "1")
	}
	if params.Changesets {
		vals.Set("changesets", "1")
	}
	if params.WikiPages {
		vals.Set("wiki_pages", "1")
	}
	if params.Messages {
		vals.Set("messages", "1")
	}
	if params.Projects {
		vals.Set("projects", "1")
	}

	if params.Limit > 0 {
		vals.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		vals.Set("offset", strconv.Itoa(params.Offset))
	}

	path := "/search.json"
	if params.ProjectID != "" {
		path = "/projects/" + url.PathEscape(params.ProjectID) + "/search.json"
	}

	var resp models.SearchResponse
	if err := s.client.Get(ctx, path, vals, &resp); err != nil {
		return nil, 0, err
	}

	return resp.Results, resp.TotalCount, nil
}
