package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/models"
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
	return FetchAll[models.Project](ctx, s.client, "/projects.json", params, "projects", limit)
}

// Get retrieves a single project by identifier.
func (s *ProjectService) Get(ctx context.Context, identifier string, includes []string) (*models.Project, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", joinStrings(includes, ","))
	}
	var resp struct {
		Project models.Project `json:"project"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/projects/%s.json", identifier), params, &resp); err != nil {
		return nil, err
	}
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
	return &resp.Project, nil
}

// Update updates an existing project.
func (s *ProjectService) Update(ctx context.Context, identifier string, update models.ProjectUpdate) error {
	body := map[string]interface{}{"project": update}
	return s.client.Put(ctx, fmt.Sprintf("/projects/%s.json", identifier), body)
}

// Delete deletes a project.
func (s *ProjectService) Delete(ctx context.Context, identifier string) error {
	return s.client.Delete(ctx, fmt.Sprintf("/projects/%s.json", identifier))
}

// Members retrieves project memberships.
func (s *ProjectService) Members(ctx context.Context, identifier string, limit, offset int) ([]models.Membership, int, error) {
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.Membership](ctx, s.client, fmt.Sprintf("/projects/%s/memberships.json", identifier), params, "memberships", limit)
}
