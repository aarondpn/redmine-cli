package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// TimeEntryService handles time entry API calls.
type TimeEntryService struct {
	client *Client
}

// List retrieves time entries matching the given filter.
func (s *TimeEntryService) List(ctx context.Context, filter models.TimeEntryFilter) ([]models.TimeEntry, int, error) {
	params := url.Values{}
	if filter.ProjectID != "" {
		params.Set("project_id", filter.ProjectID)
	}
	if filter.UserID != "" {
		params.Set("user_id", filter.UserID)
	}
	if filter.IssueID > 0 {
		params.Set("issue_id", strconv.Itoa(filter.IssueID))
	}
	if filter.From != "" {
		params.Set("from", filter.From)
	}
	if filter.To != "" {
		params.Set("to", filter.To)
	}
	if filter.ActivityID > 0 {
		params.Set("activity_id", strconv.Itoa(filter.ActivityID))
	}

	limit := filter.Limit
	if limit == 0 {
		limit = 25
	}

	return FetchAll[models.TimeEntry](ctx, s.client, "/time_entries.json", params, "time_entries", limit)
}

// Get retrieves a single time entry.
func (s *TimeEntryService) Get(ctx context.Context, id int) (*models.TimeEntry, error) {
	var resp struct {
		TimeEntry models.TimeEntry `json:"time_entry"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/time_entries/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.TimeEntry, nil
}

// Create creates a new time entry.
func (s *TimeEntryService) Create(ctx context.Context, entry models.TimeEntryCreate) (*models.TimeEntry, error) {
	body := map[string]interface{}{"time_entry": entry}
	var resp struct {
		TimeEntry models.TimeEntry `json:"time_entry"`
	}
	if err := s.client.Post(ctx, "/time_entries.json", body, &resp); err != nil {
		return nil, err
	}
	return &resp.TimeEntry, nil
}

// Update updates an existing time entry.
func (s *TimeEntryService) Update(ctx context.Context, id int, update models.TimeEntryUpdate) error {
	body := map[string]interface{}{"time_entry": update}
	return s.client.Put(ctx, fmt.Sprintf("/time_entries/%d.json", id), body)
}

// Delete deletes a time entry.
func (s *TimeEntryService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/time_entries/%d.json", id))
}
