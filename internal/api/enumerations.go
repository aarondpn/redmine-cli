package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// EnumerationService handles enumeration-related API calls.
type EnumerationService struct {
	client *Client
}

// list fetches /enumerations/<kind>.json and decodes the array stored under
// the same key. Redmine ignores project_id on this endpoint, so the result is
// always the shared (global) set.
func (s *EnumerationService) list(ctx context.Context, kind string) ([]models.Enumeration, error) {
	var resp map[string]json.RawMessage
	if err := s.client.Get(ctx, "/enumerations/"+kind+".json", nil, &resp); err != nil {
		return nil, err
	}
	items := []models.Enumeration{}
	if raw, ok := resp[kind]; ok {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", kind, err)
		}
	}
	for i := range items {
		items[i].NormalizeCollections()
	}
	return items, nil
}

// TimeEntryActivities retrieves all shared time entry activities.
func (s *EnumerationService) TimeEntryActivities(ctx context.Context) ([]models.Enumeration, error) {
	return s.list(ctx, "time_entry_activities")
}

// IssuePriorities retrieves all issue priorities.
func (s *EnumerationService) IssuePriorities(ctx context.Context) ([]models.Enumeration, error) {
	return s.list(ctx, "issue_priorities")
}

// DocumentCategories retrieves all document categories.
func (s *EnumerationService) DocumentCategories(ctx context.Context) ([]models.Enumeration, error) {
	return s.list(ctx, "document_categories")
}
