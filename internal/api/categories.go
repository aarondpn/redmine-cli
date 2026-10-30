package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// CategoryService handles issue category-related API calls.
type CategoryService struct {
	client *Client
}

// List retrieves issue categories for a project.
func (s *CategoryService) List(ctx context.Context, projectID string) ([]models.IssueCategory, int, error) {
	seg, err := projectSegment(projectID)
	if err != nil {
		return nil, 0, err
	}
	path := fmt.Sprintf("/projects/%s/issue_categories.json", seg)
	return FetchAll[models.IssueCategory](ctx, s.client, path, nil, "issue_categories", 0)
}

// Get retrieves a single issue category by ID.
func (s *CategoryService) Get(ctx context.Context, id int) (*models.IssueCategory, error) {
	var resp struct {
		IssueCategory models.IssueCategory `json:"issue_category"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/issue_categories/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.IssueCategory, nil
}

// Create creates an issue category in a project.
func (s *CategoryService) Create(ctx context.Context, projectID string, c models.IssueCategoryCreate) (*models.IssueCategory, error) {
	seg, err := projectSegment(projectID)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{"issue_category": c}
	var resp struct {
		IssueCategory models.IssueCategory `json:"issue_category"`
	}
	if err := s.client.Post(ctx, fmt.Sprintf("/projects/%s/issue_categories.json", seg), body, &resp); err != nil {
		return nil, err
	}
	return &resp.IssueCategory, nil
}

// Update updates an existing issue category.
func (s *CategoryService) Update(ctx context.Context, id int, u models.IssueCategoryUpdate) error {
	body := map[string]interface{}{"issue_category": u}
	return s.client.Put(ctx, fmt.Sprintf("/issue_categories/%d.json", id), body)
}

// Delete deletes an issue category. When reassignToID > 0 the category's
// issues are moved to that category; Redmine silently ignores an ID that is
// unknown or belongs to another project, so callers must validate it.
func (s *CategoryService) Delete(ctx context.Context, id, reassignToID int) error {
	path := fmt.Sprintf("/issue_categories/%d.json", id)
	if reassignToID > 0 {
		q := url.Values{}
		q.Set("reassign_to_id", strconv.Itoa(reassignToID))
		path += "?" + q.Encode()
	}
	return s.client.Delete(ctx, path)
}
