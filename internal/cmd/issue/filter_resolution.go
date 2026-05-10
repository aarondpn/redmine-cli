package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
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

func resolveIssueAssigneeFilter(ctx context.Context, client *api.Client, assignee string, printer output.Printer) (string, error) {
	if assignee == "" || assignee == "me" {
		return assignee, nil
	}

	if _, err := strconv.Atoi(assignee); err == nil {
		return assignee, nil
	}

	id, err := resolver.ResolveAssignee(ctx, client, assignee)
	if err != nil {
		if resolver.IsNameResolutionPermissionError(err) {
			printer.Warning("Could not resolve --assignee by name because user lookup requires admin privileges; ignoring the assignee filter. Use a numeric user ID or 'me' instead.")
			return "", nil
		}
		return "", fmt.Errorf("resolving assignee: %w", err)
	}

	return strconv.Itoa(id), nil
}
