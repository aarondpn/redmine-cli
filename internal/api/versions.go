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
	seg, err := projectSegment(projectID)
	if err != nil {
		return nil, 0, err
	}
	path := fmt.Sprintf("/projects/%s/versions.json", seg)
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	versions, total, err := FetchAll[models.Version](ctx, s.client, path, params, "versions", limit)
	if err != nil {
		return nil, 0, err
	}
	for i := range versions {
		versions[i].NormalizeCollections()
	}
	return versions, total, nil
}

// Get retrieves a single version by ID.
func (s *VersionService) Get(ctx context.Context, id int) (*models.Version, error) {
	var resp struct {
		Version models.Version `json:"version"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/versions/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	resp.Version.NormalizeCollections()
	return &resp.Version, nil
}

// Create creates a version for a project.
func (s *VersionService) Create(ctx context.Context, projectID string, version models.VersionCreate) (*models.Version, error) {
	seg, err := projectSegment(projectID)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{"version": version}
	var resp struct {
		Version models.Version `json:"version"`
	}
	if err := s.client.Post(ctx, fmt.Sprintf("/projects/%s/versions.json", seg), body, &resp); err != nil {
		return nil, err
	}
	resp.Version.NormalizeCollections()
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
	seg, err := projectSegment(projectID)
	if err != nil {
		return nil, false, err
	}
	path := fmt.Sprintf("/projects/%s/versions.json", seg)
	versions, hasMore, err := FetchAllFiltered[models.Version](ctx, s.client, path, nil, "versions", need, filter)
	if err != nil {
		return nil, false, err
	}
	for i := range versions {
		versions[i].NormalizeCollections()
	}
	return versions, hasMore, nil
}
