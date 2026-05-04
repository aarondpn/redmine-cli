package api

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// RoleService handles role-related API calls.
type RoleService struct {
	client *Client
}

// List retrieves all roles.
func (s *RoleService) List(ctx context.Context) ([]models.Role, error) {
	var resp struct {
		Roles []models.Role `json:"roles"`
	}
	if err := s.client.Get(ctx, "/roles.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Roles, nil
}

// Get retrieves a single role by ID.
func (s *RoleService) Get(ctx context.Context, id int) (*models.Role, error) {
	var resp struct {
		Role models.Role `json:"role"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/roles/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Role, nil
}
