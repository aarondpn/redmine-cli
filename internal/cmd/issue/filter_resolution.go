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

func resolveIssueQueryFilter(ctx context.Context, client *api.Client, query string, queryID int, projectIdentifier string) (int, error) {
	if queryID > 0 && query != "" {
		return 0, fmt.Errorf("--query and --query-id are mutually exclusive; pick one")
	}
	if queryID > 0 {
		return queryID, nil
	}
	if query == "" {
		return 0, nil
	}
	id, err := resolver.ResolveQuery(ctx, client, query, projectIdentifier)
	if err != nil {
		return 0, fmt.Errorf("resolving query: %w", err)
	}
	return id, nil
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
