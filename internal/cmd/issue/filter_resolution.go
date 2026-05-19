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

// resolveIssueUserFilter accepts "me", a numeric ID, or a name/login. When
// user lookup is forbidden (non-admin user trying to resolve a name) we warn
// via the printer and drop the filter rather than abort — same behavior for
// --assignee and --author. flagName is used in the warning text.
func resolveIssueUserFilter(ctx context.Context, client *api.Client, input string, printer output.Printer, flagName string) (string, error) {
	if input == "" || input == "me" {
		return input, nil
	}
	if _, err := strconv.Atoi(input); err == nil {
		return input, nil
	}
	id, err := resolver.ResolveAssignee(ctx, client, input)
	if err != nil {
		if resolver.IsNameResolutionPermissionError(err) {
			printer.Warning(fmt.Sprintf("Could not resolve --%s by name because user lookup requires admin privileges; ignoring the %s filter. Use a numeric user ID or 'me' instead.", flagName, flagName))
			return "", nil
		}
		return "", fmt.Errorf("resolving %s: %w", flagName, err)
	}
	return strconv.Itoa(id), nil
}

func resolveIssueAssigneeFilter(ctx context.Context, client *api.Client, assignee string, printer output.Printer) (string, error) {
	return resolveIssueUserFilter(ctx, client, assignee, printer, "assignee")
}

func resolveIssueAuthorFilter(ctx context.Context, client *api.Client, author string, printer output.Printer) (string, error) {
	return resolveIssueUserFilter(ctx, client, author, printer, "author")
}

// resolveUserList resolves each input (name, login, ID, or 'me') to a numeric
// user ID. Used by flags that accept multiple users like --watcher.
func resolveUserList(ctx context.Context, client *api.Client, inputs []string, label string) ([]int, error) {
	ids := make([]int, 0, len(inputs))
	for _, raw := range inputs {
		if raw == "" {
			continue
		}
		id, err := resolver.ResolveAssignee(ctx, client, raw)
		if err != nil {
			return nil, fmt.Errorf("resolving --%s %q: %w", label, raw, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
