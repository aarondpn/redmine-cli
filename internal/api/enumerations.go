package api

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// EnumerationService handles enumeration-related API calls.
type EnumerationService struct {
	client *Client
}

// TimeEntryActivities retrieves all time entry activities.
func (s *EnumerationService) TimeEntryActivities(ctx context.Context) ([]models.Enumeration, error) {
	var resp struct {
		Activities []models.Enumeration `json:"time_entry_activities"`
	}
	if err := s.client.Get(ctx, "/enumerations/time_entry_activities.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Activities, nil
}

// IssuePriorities retrieves all issue priorities.
func (s *EnumerationService) IssuePriorities(ctx context.Context) ([]models.Enumeration, error) {
	var resp struct {
		Priorities []models.Enumeration `json:"issue_priorities"`
	}
	if err := s.client.Get(ctx, "/enumerations/issue_priorities.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Priorities, nil
}
