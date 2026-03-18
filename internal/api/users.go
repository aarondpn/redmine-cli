package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// UserService handles user-related API calls.
type UserService struct {
	client *Client
}

// List retrieves users matching the given filter.
func (s *UserService) List(ctx context.Context, filter models.UserFilter) ([]models.User, int, error) {
	params := url.Values{}
	if filter.Status != "" {
		statusMap := map[string]string{
			"active":     "1",
			"registered": "2",
			"locked":     "3",
		}
		if v, ok := statusMap[filter.Status]; ok {
			params.Set("status", v)
		} else {
			params.Set("status", filter.Status)
		}
	}
	if filter.Name != "" {
		params.Set("name", filter.Name)
	}
	if filter.GroupID > 0 {
		params.Set("group_id", strconv.Itoa(filter.GroupID))
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}

	return FetchAll[models.User](ctx, s.client, "/users.json", params, "users", filter.Limit)
}

// Get retrieves a single user by ID.
func (s *UserService) Get(ctx context.Context, id int) (*models.User, error) {
	var resp struct {
		User models.User `json:"user"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/users/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// Current retrieves the currently authenticated user.
func (s *UserService) Current(ctx context.Context) (*models.User, error) {
	var resp struct {
		User models.User `json:"user"`
	}
	if err := s.client.Get(ctx, "/users/current.json", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// Create creates a new user.
func (s *UserService) Create(ctx context.Context, user models.UserCreate) (*models.User, error) {
	body := map[string]interface{}{"user": user}
	var resp struct {
		User models.User `json:"user"`
	}
	if err := s.client.Post(ctx, "/users.json", body, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// Update updates an existing user.
func (s *UserService) Update(ctx context.Context, id int, update models.UserUpdate) error {
	body := map[string]interface{}{"user": update}
	return s.client.Put(ctx, fmt.Sprintf("/users/%d.json", id), body)
}

// Delete deletes a user.
func (s *UserService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/users/%d.json", id))
}
