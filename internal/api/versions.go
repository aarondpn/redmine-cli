package api

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// VersionService handles version-related API calls.
type VersionService struct {
	client *Client
}

// List retrieves all versions for a project.
func (s *VersionService) List(ctx context.Context, projectID string, limit int) ([]models.Version, int, error) {
	path := fmt.Sprintf("/projects/%s/versions.json", projectID)
	if limit == 0 {
		limit = 100
	}
	return FetchAll[models.Version](ctx, s.client, path, nil, "versions", limit)
}
