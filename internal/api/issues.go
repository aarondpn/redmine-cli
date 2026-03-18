package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// IssueService handles issue-related API calls.
type IssueService struct {
	client *Client
}

// List retrieves issues matching the given filter.
func (s *IssueService) List(ctx context.Context, filter models.IssueFilter) ([]models.Issue, int, error) {
	params := url.Values{}
	if filter.ProjectID != "" {
		params.Set("project_id", filter.ProjectID)
	}
	if filter.TrackerID > 0 {
		params.Set("tracker_id", strconv.Itoa(filter.TrackerID))
	}
	if filter.StatusID != "" {
		params.Set("status_id", filter.StatusID)
	}
	if filter.AssignedToID != "" {
		params.Set("assigned_to_id", filter.AssignedToID)
	}
	if filter.FixedVersionID > 0 {
		params.Set("fixed_version_id", strconv.Itoa(filter.FixedVersionID))
	}
	if filter.Sort != "" {
		params.Set("sort", filter.Sort)
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}

	return FetchAll[models.Issue](ctx, s.client, "/issues.json", params, "issues", filter.Limit)
}

// Get retrieves a single issue by ID.
func (s *IssueService) Get(ctx context.Context, id int, includes []string) (*models.Issue, error) {
	params := url.Values{}
	if len(includes) > 0 {
		params.Set("include", joinStrings(includes, ","))
	}

	var resp struct {
		Issue models.Issue `json:"issue"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/issues/%d.json", id), params, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

// Create creates a new issue.
func (s *IssueService) Create(ctx context.Context, issue models.IssueCreate) (*models.Issue, error) {
	body := map[string]interface{}{"issue": issue}
	var resp struct {
		Issue models.Issue `json:"issue"`
	}
	if err := s.client.Post(ctx, "/issues.json", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

// Update updates an existing issue.
func (s *IssueService) Update(ctx context.Context, id int, update models.IssueUpdate) error {
	body := map[string]interface{}{"issue": update}
	return s.client.Put(ctx, fmt.Sprintf("/issues/%d.json", id), body)
}

// Delete deletes an issue.
func (s *IssueService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/issues/%d.json", id))
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
