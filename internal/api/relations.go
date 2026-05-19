package api

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// RelationService handles issue relation API calls.
type RelationService struct {
	client *Client
}

// ListByIssue returns relations attached to the given issue.
func (s *RelationService) ListByIssue(ctx context.Context, issueID int) ([]models.IssueRelation, error) {
	var resp struct {
		Relations []models.IssueRelation `json:"relations"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/issues/%d/relations.json", issueID), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Relations, nil
}

// Get returns a single relation by its ID.
func (s *RelationService) Get(ctx context.Context, relationID int) (*models.IssueRelation, error) {
	var resp struct {
		Relation models.IssueRelation `json:"relation"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/relations/%d.json", relationID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Relation, nil
}

// Create creates a new relation on the given issue.
func (s *RelationService) Create(ctx context.Context, issueID int, payload models.IssueRelationCreate) (*models.IssueRelation, error) {
	body := map[string]interface{}{"relation": payload}
	var resp struct {
		Relation models.IssueRelation `json:"relation"`
	}
	if err := s.client.Post(ctx, fmt.Sprintf("/issues/%d/relations.json", issueID), body, &resp); err != nil {
		return nil, err
	}
	return &resp.Relation, nil
}

// Delete removes a relation by its ID.
func (s *RelationService) Delete(ctx context.Context, relationID int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/relations/%d.json", relationID))
}
