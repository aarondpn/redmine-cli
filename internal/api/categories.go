package api

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// CategoryService handles issue category-related API calls.
type CategoryService struct {
	client *Client
}

// List retrieves issue categories for a project.
func (s *CategoryService) List(ctx context.Context, projectID string) ([]models.IssueCategory, int, error) {
	path := fmt.Sprintf("/projects/%s/issue_categories.json", projectID)
	return FetchAll[models.IssueCategory](ctx, s.client, path, nil, "issue_categories", 0)
}
