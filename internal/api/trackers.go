package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
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

// Get retrieves a single tracker by ID by resolving it from the tracker list
// endpoint, which is the only tracker endpoint Redmine exposes.
func (s *TrackerService) Get(ctx context.Context, id int) (*models.Tracker, error) {
	trackers, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range trackers {
		if trackers[i].ID == id {
			return &trackers[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Errors:     []string{fmt.Sprintf("tracker %d not found", id)},
		URL:        fmt.Sprintf("%s/trackers.json", s.client.baseURL),
	}
}
