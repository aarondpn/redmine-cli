package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func resolveIssueStatusFilter(ctx context.Context, client *api.Client, status string) (string, error) {
	if status == "" || status == "open" || status == "closed" || status == "*" {
		return status, nil
	}

	id, err := resolver.ResolveStatus(ctx, client, status)
	if err != nil {
		return "", fmt.Errorf("resolving status: %w", err)
	}
	return strconv.Itoa(id), nil
}

// resolveIssueQueryFilter validates the --query / --query-id flags and
// returns the saved query ID to thread into the issues list request. Mutual
// exclusion is enforced by cobra at flag-parse time, so callers reaching
// here have at most one of the two set. A non-positive --query-id is
// rejected because Redmine's query_id is a positive integer; treating zero
// as "no filter" would silently swallow user typos.
func resolveIssueQueryFilter(ctx context.Context, client *api.Client, query string, queryID int, queryIDChanged bool, projectIdentifier string) (int, error) {
	if queryIDChanged {
		if queryID <= 0 {
			return 0, fmt.Errorf("--query-id must be a positive integer, got %d", queryID)
		}
		return queryID, nil
	}
	if query == "" {
		return 0, nil
	}
	q, err := resolver.ResolveQuery(ctx, client, query, projectIdentifier)
	if err != nil {
		return 0, fmt.Errorf("resolving query: %w", err)
	}
	return q.ID, nil
}

// resolveIssuePriorityFilter accepts a numeric ID or a name; names are
// resolved via /enumerations/issue_priorities.json. Returns the stringified
// ID expected by the Redmine REST API filter.
func resolveIssuePriorityFilter(ctx context.Context, client *api.Client, priority string) (string, error) {
	if priority == "" {
		return "", nil
	}
	if _, err := strconv.Atoi(priority); err == nil {
		return priority, nil
	}
	id, err := resolver.ResolvePriority(ctx, client, priority)
	if err != nil {
		return "", fmt.Errorf("resolving priority: %w", err)
	}
	return strconv.Itoa(id), nil
}

// resolveIssueUserFilter accepts "me", a numeric ID, or a name/login. Names
// are matched against the memberships of project (when set) before the
// admin-only users endpoint. A name that cannot be resolved is an error for
// both --assignee and --author: silently dropping the filter would return
// unfiltered issues that look like filtered ones. flagName is used in the
// error text.
func resolveIssueUserFilter(ctx context.Context, client *api.Client, input, project, flagName string, allowGroups bool) (string, error) {
	if input == "" || input == "me" {
		return input, nil
	}
	if _, err := strconv.Atoi(input); err == nil {
		return input, nil
	}
	resolve := resolver.ResolveUserInProject
	if allowGroups {
		resolve = resolver.ResolveAssigneeInProject
	}
	id, err := resolve(ctx, client, input, project)
	if err != nil {
		if resolver.IsNameResolutionPermissionError(err) && project == "" {
			return "", fmt.Errorf("resolving %s: %w. Pass --project to match project members by name", flagName, err)
		}
		return "", fmt.Errorf("resolving %s: %w", flagName, err)
	}
	return strconv.Itoa(id), nil
}

// resolveIssueAssigneeFilter resolves --assignee for issue listings. Besides
// the user forms it accepts group names and "none", which maps to Redmine's
// "no assignee" operator (assigned_to_id=!*).
func resolveIssueAssigneeFilter(ctx context.Context, client *api.Client, assignee, project string) (string, error) {
	if resolver.IsNone(assignee) {
		return resolver.UnassignedFilter, nil
	}
	return resolveIssueUserFilter(ctx, client, assignee, project, "assignee", true)
}

func resolveIssueAuthorFilter(ctx context.Context, client *api.Client, author, project string) (string, error) {
	return resolveIssueUserFilter(ctx, client, author, project, "author", false)
}

// resolveAssigneeValue resolves an --assignee value for commands that set
// the assignee. "none" yields 0, which the update path sends as an empty
// assigned_to_id to clear the assignee. Names are matched against the
// project memberships principals is bound to (users and groups) before the
// users endpoint.
func resolveAssigneeValue(principals *resolver.PrincipalResolver, input string) (int, error) {
	if resolver.IsNone(input) {
		return 0, nil
	}
	return principals.Assignee(input)
}

// resolveIssuePrincipal resolves a user or group in the context of an
// existing issue. Names are matched against the issue's project memberships,
// so the issue is fetched only when a name lookup is actually needed (never
// for numeric IDs or "me").
func resolveIssuePrincipal(ctx context.Context, client *api.Client, issueID int, input string) (int, error) {
	var project string
	if resolver.NeedsPrincipalLookup(input) {
		issue, err := ops.GetIssue(ctx, client, ops.GetIssueInput{ID: issueID})
		if err != nil {
			return 0, fmt.Errorf("failed to fetch issue for name resolution: %w", err)
		}
		project = strconv.Itoa(issue.Project.ID)
	}
	return resolver.ResolveAssigneeInProject(ctx, client, input, project)
}
