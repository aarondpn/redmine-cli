package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// ProjectService handles project-related API calls.
type ProjectService struct {
	client *Client
}

// List retrieves all projects.
func (s *ProjectService) List(ctx context.Context, includes []string, limit, offset int) ([]models.Project, int, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", joinStrings(includes, ","))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	projects, total, err := FetchAll[models.Project](ctx, s.client, "/projects.json", params, "projects", limit)
	if err != nil {
		return nil, 0, err
	}
	for i := range projects {
		projects[i].NormalizeCollections()
	}
	return projects, total, nil
}

// Get retrieves a single project by identifier.
func (s *ProjectService) Get(ctx context.Context, identifier string, includes []string) (*models.Project, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", joinStrings(includes, ","))
	}
	seg, err := projectSegment(identifier)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Project models.Project `json:"project"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/projects/%s.json", seg), params, &resp); err != nil {
		return nil, err
	}
	resp.Project.NormalizeCollections()
	return &resp.Project, nil
}

// Create creates a new project.
func (s *ProjectService) Create(ctx context.Context, project models.ProjectCreate) (*models.Project, error) {
	body := map[string]interface{}{"project": project}
	var resp struct {
		Project models.Project `json:"project"`
	}
	if err := s.client.Post(ctx, "/projects.json", body, &resp); err != nil {
		return nil, err
	}
	resp.Project.NormalizeCollections()
	return &resp.Project, nil
}

// Update updates an existing project.
func (s *ProjectService) Update(ctx context.Context, identifier string, update models.ProjectUpdate) error {
	seg, err := projectSegment(identifier)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"project": update}
	return s.client.Put(ctx, fmt.Sprintf("/projects/%s.json", seg), body)
}

// Delete deletes a project.
func (s *ProjectService) Delete(ctx context.Context, identifier string) error {
	seg, err := projectSegment(identifier)
	if err != nil {
		return err
	}
	return s.client.Delete(ctx, fmt.Sprintf("/projects/%s.json", seg))
}

// Archive archives a project. Requires Redmine 5.0+.
func (s *ProjectService) Archive(ctx context.Context, identifier string) error {
	seg, err := projectSegment(identifier)
	if err != nil {
		return err
	}
	return s.client.Put(ctx, fmt.Sprintf("/projects/%s/archive.json", seg), nil)
}

// Unarchive unarchives a previously-archived project. Requires Redmine 5.0+.
func (s *ProjectService) Unarchive(ctx context.Context, identifier string) error {
	seg, err := projectSegment(identifier)
	if err != nil {
		return err
	}
	return s.client.Put(ctx, fmt.Sprintf("/projects/%s/unarchive.json", seg), nil)
}

// Members retrieves project memberships.
func (s *ProjectService) Members(ctx context.Context, identifier string, limit, offset int) ([]models.Membership, int, error) {
	seg, err := projectSegment(identifier)
	if err != nil {
		return nil, 0, err
	}
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.Membership](ctx, s.client, fmt.Sprintf("/projects/%s/memberships.json", seg), params, "memberships", limit)
}
