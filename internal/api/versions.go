package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// VersionService handles version-related API calls.
type VersionService struct {
	client *Client
}

// List retrieves versions for a project. If limit is 0, all versions are fetched.
func (s *VersionService) List(ctx context.Context, projectID string, limit, offset int) ([]models.Version, int, error) {
	path := fmt.Sprintf("/projects/%s/versions.json", projectID)
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.Version](ctx, s.client, path, params, "versions", limit)
}

// Get retrieves a single version by ID.
func (s *VersionService) Get(ctx context.Context, id int) (*models.Version, error) {
	var resp struct {
		Version models.Version `json:"version"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/versions/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Version, nil
}

// ListFiltered pages through versions for a project, keeping only those that
// match the filter function, and returns once need results have been collected
// (or there are no more pages). If need is 0, all matching versions are returned.
// The hasMore return indicates whether additional matches may exist beyond what was collected.
func (s *VersionService) ListFiltered(ctx context.Context, projectID string, need int, filter func(models.Version) bool) ([]models.Version, bool, error) {
	path := fmt.Sprintf("/projects/%s/versions.json", projectID)
	return FetchAllFiltered[models.Version](ctx, s.client, path, nil, "versions", need, filter)
}
