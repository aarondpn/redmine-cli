package api

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// StatusService handles issue status API calls.
type StatusService struct {
	client *Client
}

// List retrieves all issue statuses.
func (s *StatusService) List(ctx context.Context) ([]models.IssueStatus, error) {
	var resp struct {
		Statuses []models.IssueStatus `json:"issue_statuses"`
	}
	if err := s.client.Get(ctx, "/issue_statuses.json", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Statuses, nil
}
