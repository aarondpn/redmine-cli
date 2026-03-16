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

// List retrieves versions for a project. If limit is 0, all versions are fetched.
func (s *VersionService) List(ctx context.Context, projectID string, limit int) ([]models.Version, int, error) {
	path := fmt.Sprintf("/projects/%s/versions.json", projectID)
	return FetchAll[models.Version](ctx, s.client, path, nil, "versions", limit)
}
