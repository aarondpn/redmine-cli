package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// WikiService handles wiki-related API calls.
type WikiService struct {
	client *Client
}

// List retrieves the wiki page index for a project.
func (s *WikiService) List(ctx context.Context, projectID string, limit, offset int) ([]models.WikiPageIndex, int, error) {
	path := fmt.Sprintf("/projects/%s/wiki/index.json", url.PathEscape(projectID))
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.WikiPageIndex](ctx, s.client, path, params, "wiki_pages", limit)
}

// Get retrieves a single wiki page by title.
func (s *WikiService) Get(ctx context.Context, projectID, page string, includes []string) (*models.WikiPage, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", joinStrings(includes, ","))
	}
	var resp struct {
		WikiPage models.WikiPage `json:"wiki_page"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/projects/%s/wiki/%s.json", url.PathEscape(projectID), url.PathEscape(page)), params, &resp); err != nil {
		return nil, err
	}
	return &resp.WikiPage, nil
}

// GetVersion retrieves a specific version of a wiki page.
func (s *WikiService) GetVersion(ctx context.Context, projectID, page string, version int) (*models.WikiPage, error) {
	var resp struct {
		WikiPage models.WikiPage `json:"wiki_page"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/projects/%s/wiki/%s/%d.json", url.PathEscape(projectID), url.PathEscape(page), version), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.WikiPage, nil
}

// Create creates a new wiki page (or overwrites an existing one).
func (s *WikiService) Create(ctx context.Context, projectID, page string, wiki models.WikiPageCreate) error {
	body := map[string]interface{}{"wiki_page": wiki}
	return s.client.Put(ctx, fmt.Sprintf("/projects/%s/wiki/%s.json", url.PathEscape(projectID), url.PathEscape(page)), body)
}

// Update updates an existing wiki page.
func (s *WikiService) Update(ctx context.Context, projectID, page string, update models.WikiPageUpdate) error {
	body := map[string]interface{}{"wiki_page": update}
	return s.client.Put(ctx, fmt.Sprintf("/projects/%s/wiki/%s.json", url.PathEscape(projectID), url.PathEscape(page)), body)
}

// Delete deletes a wiki page.
func (s *WikiService) Delete(ctx context.Context, projectID, page string) error {
	return s.client.Delete(ctx, fmt.Sprintf("/projects/%s/wiki/%s.json", url.PathEscape(projectID), url.PathEscape(page)))
}
