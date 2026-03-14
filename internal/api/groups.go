package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// GroupService handles group-related API calls.
type GroupService struct {
	client *Client
}

// List retrieves groups matching the given filter.
func (s *GroupService) List(ctx context.Context, filter models.GroupFilter) ([]models.Group, int, error) {
	limit := filter.Limit
	if limit == 0 {
		limit = 25
	}

	return FetchAll[models.Group](ctx, s.client, "/groups.json", nil, "groups", limit)
}

// Get retrieves a single group by ID. includes can contain "users" and/or "memberships".
func (s *GroupService) Get(ctx context.Context, id int, includes []string) (*models.Group, error) {
	var params url.Values
	if len(includes) > 0 {
		params = url.Values{}
		params.Set("include", strings.Join(includes, ","))
	}

	var resp struct {
		Group models.Group `json:"group"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/groups/%d.json", id), params, &resp); err != nil {
		return nil, err
	}
	return &resp.Group, nil
}

// Create creates a new group.
func (s *GroupService) Create(ctx context.Context, group models.GroupCreate) (*models.Group, error) {
	body := map[string]interface{}{"group": group}
	var resp struct {
		Group models.Group `json:"group"`
	}
	if err := s.client.Post(ctx, "/groups.json", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Group, nil
}

// Update updates an existing group.
func (s *GroupService) Update(ctx context.Context, id int, update models.GroupUpdate) error {
	body := map[string]interface{}{"group": update}
	return s.client.Put(ctx, fmt.Sprintf("/groups/%d.json", id), body)
}

// Delete deletes a group.
func (s *GroupService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/groups/%d.json", id))
}

// AddUser adds a user to a group.
func (s *GroupService) AddUser(ctx context.Context, groupID, userID int) error {
	body := map[string]interface{}{"user_id": userID}
	return s.client.Post(ctx, fmt.Sprintf("/groups/%d/users.json", groupID), body, nil)
}

// RemoveUser removes a user from a group.
func (s *GroupService) RemoveUser(ctx context.Context, groupID, userID int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/groups/%d/users/%d.json", groupID, userID))
}
