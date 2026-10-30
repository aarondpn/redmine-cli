package api

import (
	"context"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// SearchService handles the Redmine search API.
type SearchService struct {
	client *Client
}

// SearchParams holds parameters for the search API call.
type SearchParams struct {
	Query       string
	ProjectID   string
	Scope       string // "all", "my_projects", "bookmarks", "subprojects"
	AllWords    *bool  // nil means Redmine's default (all words)
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

	if params.Scope != "" {
		vals.Set("scope", params.Scope)
	}
	// Redmine treats a missing all_words as true and an empty value as false,
	// so always send an explicit value.
	if params.AllWords != nil && !*params.AllWords {
		vals.Set("all_words", "")
	} else {
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
		seg, err := projectSegment(params.ProjectID)
		if err != nil {
			return nil, 0, err
		}
		path = "/projects/" + seg + "/search.json"
	}

	var resp models.SearchResponse
	if err := s.client.Get(ctx, path, vals, &resp); err != nil {
		return nil, 0, err
	}

	return resp.Results, resp.TotalCount, nil
}
