package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
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
	if filter.SubprojectID != "" {
		params.Set("subproject_id", filter.SubprojectID)
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
	if filter.AuthorID != "" {
		params.Set("author_id", filter.AuthorID)
	}
	if filter.PriorityID != "" {
		params.Set("priority_id", filter.PriorityID)
	}
	if filter.CategoryID > 0 {
		params.Set("category_id", strconv.Itoa(filter.CategoryID))
	}
	if filter.FixedVersionID > 0 {
		params.Set("fixed_version_id", strconv.Itoa(filter.FixedVersionID))
	}
	if filter.ParentID > 0 {
		params.Set("parent_id", strconv.Itoa(filter.ParentID))
	}
	if filter.IsPrivate != nil {
		params.Set("is_private", strconv.FormatBool(*filter.IsPrivate))
	}
	if filter.QueryID > 0 {
		params.Set("query_id", strconv.Itoa(filter.QueryID))
	}
	if len(filter.Includes) > 0 {
		params.Set("include", joinStrings(filter.Includes, ","))
	}
	if filter.Sort != "" {
		params.Set("sort", filter.Sort)
	}
	if filter.Offset > 0 {
		params.Set("offset", strconv.Itoa(filter.Offset))
	}
	// Extra params last so the escape hatch can override explicit fields if a
	// user knows what they're doing (e.g. raw `status_id=open|closed`).
	applyFilterParams(params, filter.ExtraParams)

	issues, total, err := FetchAll[models.Issue](ctx, s.client, "/issues.json", params, "issues", filter.Limit)
	if err != nil {
		return nil, 0, err
	}
	// The list endpoint sends included collections as arrays even when empty
	// and ignores children/watchers, so only always-present fields need help.
	for i := range issues {
		issues[i].NormalizeCollections(nil)
	}
	return issues, total, nil
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
	resp.Issue.NormalizeCollections(includes)
	return &resp.Issue, nil
}

// GetWithAllowedStatuses retrieves a single issue together with the statuses
// the current user may move it to (include=allowed_statuses). Redmine added
// that include in 5.0; older servers ignore it, in which case allowed is nil
// and reported is false. An empty, reported list means Redmine allows no
// transition at all.
func (s *IssueService) GetWithAllowedStatuses(ctx context.Context, id int) (issue *models.Issue, allowed []models.IssueStatus, reported bool, err error) {
	params := url.Values{}
	params.Set("include", "allowed_statuses")

	var resp struct {
		Issue models.Issue `json:"issue"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/issues/%d.json", id), params, &resp); err != nil {
		return nil, nil, false, err
	}
	if resp.Issue.AllowedStatuses != nil {
		allowed = resp.Issue.AllowedStatuses
		reported = true
	}
	return &resp.Issue, allowed, reported, nil
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
	resp.Issue.NormalizeCollections(nil)
	return &resp.Issue, nil
}

// Copy creates a copy of an existing issue. copy_from, link_copy,
// copy_attachments and copy_subtasks go at the top level of the body, next to
// the issue hash. The flags are sent as "1": Redmine compares link_copy and
// (6.0+) copy_attachments against the string '1' when the matching setting is
// "ask", and omitting a flag leaves the server setting in charge. The target
// project is also sent as the top-level project_id, which is what Redmine
// checks the add_issue_watchers permission against before copying watchers
// (without it watchers are never copied over the API).
func (s *IssueService) Copy(ctx context.Context, cp models.IssueCopy) (*models.Issue, error) {
	body := map[string]interface{}{
		"copy_from":  cp.CopyFrom,
		"project_id": cp.Issue.ProjectID,
		"issue":      cp.Issue,
	}
	if cp.Link {
		body["link_copy"] = "1"
	}
	if cp.CopyAttachments {
		body["copy_attachments"] = "1"
	}
	if cp.CopySubtasks {
		body["copy_subtasks"] = "1"
	}
	var resp struct {
		Issue models.Issue `json:"issue"`
	}
	if err := s.client.Post(ctx, "/issues.json", body, &resp); err != nil {
		return nil, err
	}
	resp.Issue.NormalizeCollections(nil)
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

// AddWatcher adds a user as a watcher on the given issue.
func (s *IssueService) AddWatcher(ctx context.Context, issueID, userID int) error {
	body := map[string]interface{}{"user_id": userID}
	return s.client.Post(ctx, fmt.Sprintf("/issues/%d/watchers.json", issueID), body, nil)
}

// AddWatchers adds several users as watchers in one request
// (watcher[user_ids][], accepted by every supported Redmine line, 4.2+).
// Redmine skips IDs that are not valid watchers instead of failing.
func (s *IssueService) AddWatchers(ctx context.Context, issueID int, userIDs []int) error {
	body := map[string]interface{}{"watcher": map[string]interface{}{"user_ids": userIDs}}
	return s.client.Post(ctx, fmt.Sprintf("/issues/%d/watchers.json", issueID), body, nil)
}

// RemoveWatcher removes a user as a watcher from the given issue.
func (s *IssueService) RemoveWatcher(ctx context.Context, issueID, userID int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/issues/%d/watchers/%d.json", issueID, userID))
}

// ErrWatchersNotVisible is returned by ListWatchers when Redmine omits the
// watchers of an issue the caller can see, which it does when the account
// lacks the View watchers permission. Reporting an empty list instead would
// wrongly claim the issue has no watchers.
var ErrWatchersNotVisible = errors.New("watchers not visible (requires the View watchers permission)")

// ListWatchers returns the watchers on the given issue. Redmine has no
// dedicated list endpoint; the data is fetched via include=watchers on the
// issue itself. The result is non-nil on success (empty when there are no
// watchers).
func (s *IssueService) ListWatchers(ctx context.Context, issueID int) ([]models.IDName, error) {
	issue, err := s.Get(ctx, issueID, []string{"watchers"})
	if err != nil {
		return nil, err
	}
	if issue.Watchers == nil {
		return nil, ErrWatchersNotVisible
	}
	return issue.Watchers, nil
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
