package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// UserService handles user-related API calls.
type UserService struct {
	client *Client
}

// UserStatusAll is the UserFilter.Status keyword that lists users of every
// status.
const UserStatusAll = "all"

// List retrieves users matching the given filter.
//
// Status "all" first sends an empty status=, which lists every status on
// Redmine 4.2 to 5.1. Redmine 6.0+ answers that with 422 ("Status cannot be
// blank") and takes status=* instead, which 4.2 in turn answers with an
// empty list, so on a 422 the request is repeated once with status=*. A raw
// status in ExtraParams disables the retry: the user's value wins.
func (s *UserService) List(ctx context.Context, filter models.UserFilter) ([]models.User, int, error) {
	params := url.Values{}
	switch filter.Status {
	case "":
	case UserStatusAll:
		params.Set("status", "")
	default:
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
	if len(filter.Includes) > 0 {
		params.Set("include", joinStrings(filter.Includes, ","))
	}
	if filter.Sort != "" {
		params.Set("sort", filter.Sort)
	}
	applyFilterParams(params, filter.ExtraParams)

	users, total, err := FetchAll[models.User](ctx, s.client, "/users.json", params, "users", filter.Limit)
	var apiErr *APIError
	_, statusOverridden := filter.ExtraParams["status"]
	if filter.Status == UserStatusAll && !statusOverridden && errors.As(err, &apiErr) && apiErr.IsValidationError() {
		s.client.debugLog.Printf("Users: 422 on status=, retrying with status=*")
		params.Set("status", "*")
		users, total, err = FetchAll[models.User](ctx, s.client, "/users.json", params, "users", filter.Limit)
	}
	if err != nil {
		return nil, 0, err
	}
	for i := range users {
		users[i].NormalizeCollections()
	}
	return users, total, nil
}

// Get retrieves a single user by ID. Optional includes can request
// memberships and groups (Redmine 2.1+).
func (s *UserService) Get(ctx context.Context, id int, includes []string) (*models.User, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", joinStrings(includes, ","))
	}
	var resp struct {
		User models.User `json:"user"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/users/%d.json", id), params, &resp); err != nil {
		return nil, err
	}
	resp.User.NormalizeCollections()
	return &resp.User, nil
}

// Current retrieves the currently authenticated user via /users/current.json.
// Optional includes accept the same values as Get.
func (s *UserService) Current(ctx context.Context, includes []string) (*models.User, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", joinStrings(includes, ","))
	}
	var resp struct {
		User models.User `json:"user"`
	}
	if err := s.client.Get(ctx, "/users/current.json", params, &resp); err != nil {
		return nil, err
	}
	resp.User.NormalizeCollections()
	return &resp.User, nil
}

// userBody builds a users or my-account write body: the user attributes
// under "user" and preferences, when given, as the sibling "pref" object.
func userBody(user any, pref models.UserPreferences) map[string]interface{} {
	body := map[string]interface{}{"user": user}
	if len(pref) > 0 {
		body["pref"] = pref
	}
	return body
}

// Create creates a new user. When sendInformation is true, the server emails
// the new account holder; the flag is encoded as a sibling of the "user"
// object, not nested inside it, as is user.Pref ("pref").
func (s *UserService) Create(ctx context.Context, user models.UserCreate, sendInformation bool) (*models.User, error) {
	body := userBody(user, user.Pref)
	if sendInformation {
		body["send_information"] = true
	}
	var resp struct {
		User models.User `json:"user"`
	}
	if err := s.client.Post(ctx, "/users.json", body, &resp); err != nil {
		return nil, err
	}
	resp.User.NormalizeCollections()
	return &resp.User, nil
}

// Update updates an existing user. update.Pref is sent as the sibling "pref"
// object.
func (s *UserService) Update(ctx context.Context, id int, update models.UserUpdate) error {
	body := userBody(update, update.Pref)
	return s.client.Put(ctx, fmt.Sprintf("/users/%d.json", id), body)
}

// Delete deletes a user.
func (s *UserService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/users/%d.json", id))
}

// MyAccountService handles /my/account.json calls. Unlike /users/* it is the
// only user-write endpoint that does not require admin privileges, and its GET
// response includes api_key and custom_fields.
type MyAccountService struct {
	client *Client
}

// Get retrieves the authenticated user's account, including api_key and
// custom_fields.
func (s *MyAccountService) Get(ctx context.Context) (*models.User, error) {
	var resp struct {
		User models.User `json:"user"`
	}
	if err := s.client.Get(ctx, "/my/account.json", nil, &resp); err != nil {
		return nil, err
	}
	resp.User.NormalizeCollections()
	return &resp.User, nil
}

// Update updates the authenticated user's own account. update.Pref is sent
// as the sibling "pref" object.
func (s *MyAccountService) Update(ctx context.Context, update models.MyAccountUpdate) error {
	body := userBody(update, update.Pref)
	return s.client.Put(ctx, "/my/account.json", body)
}
