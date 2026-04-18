package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// MembershipService handles membership-related API calls.
type MembershipService struct {
	client *Client
}

// List retrieves memberships for a project.
func (s *MembershipService) List(ctx context.Context, projectID string, limit, offset int) ([]models.Membership, int, error) {
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.Membership](ctx, s.client, fmt.Sprintf("/projects/%s/memberships.json", projectID), params, "memberships", limit)
}

// Get retrieves a single membership by ID.
func (s *MembershipService) Get(ctx context.Context, id int) (*models.Membership, error) {
	var resp struct {
		Membership models.Membership `json:"membership"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/memberships/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Membership, nil
}

// Create creates a new membership in a project.
func (s *MembershipService) Create(ctx context.Context, projectID string, membership models.MembershipCreate) (*models.Membership, error) {
	body := map[string]interface{}{"membership": membership}
	var resp struct {
		Membership models.Membership `json:"membership"`
	}
	if err := s.client.Post(ctx, fmt.Sprintf("/projects/%s/memberships.json", projectID), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Membership, nil
}

// Update updates an existing membership.
func (s *MembershipService) Update(ctx context.Context, id int, update models.MembershipUpdate) error {
	body := map[string]interface{}{"membership": update}
	return s.client.Put(ctx, fmt.Sprintf("/memberships/%d.json", id), body)
}

// Delete deletes a membership.
func (s *MembershipService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/memberships/%d.json", id))
}
