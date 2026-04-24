package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
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

// Create creates a version for a project.
func (s *VersionService) Create(ctx context.Context, projectID string, version models.VersionCreate) (*models.Version, error) {
	body := map[string]interface{}{"version": version}
	var resp struct {
		Version models.Version `json:"version"`
	}
	if err := s.client.Post(ctx, fmt.Sprintf("/projects/%s/versions.json", projectID), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Version, nil
}

// Update updates an existing version.
func (s *VersionService) Update(ctx context.Context, id int, update models.VersionUpdate) error {
	body := map[string]interface{}{"version": update}
	return s.client.Put(ctx, fmt.Sprintf("/versions/%d.json", id), body)
}

// Delete deletes a version.
func (s *VersionService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/versions/%d.json", id))
}

// ListFiltered pages through versions for a project, keeping only those that
// match the filter function, and returns once need results have been collected
// (or there are no more pages). If need is 0, all matching versions are returned.
// The hasMore return indicates whether additional matches may exist beyond what was collected.
func (s *VersionService) ListFiltered(ctx context.Context, projectID string, need int, filter func(models.Version) bool) ([]models.Version, bool, error) {
	path := fmt.Sprintf("/projects/%s/versions.json", projectID)
	return FetchAllFiltered[models.Version](ctx, s.client, path, nil, "versions", need, filter)
}
