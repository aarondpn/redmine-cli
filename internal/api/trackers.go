package api

import (
	"context"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// TrackerService handles tracker-related API calls.
type TrackerService struct {
	client *Client
}

// List retrieves all trackers.
func (s *TrackerService) List(ctx context.Context) ([]models.Tracker, error) {
	var resp struct {
		Trackers []models.Tracker `json:"trackers"`
	}
	if err := s.client.Get(ctx, "/trackers.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Trackers, nil
}
