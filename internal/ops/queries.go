package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListQueriesInput struct {
	Type   string `json:"type,omitempty" jsonschema:"Kind of saved query: 'issue' (default) or 'time_entry'. Pass the id of a time_entry query to list_time_entries as query_id."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip (pagination)."`
}

// Saved query kinds accepted by ListQueriesInput.Type and `queries list
// --type`.
const (
	QueryTypeIssue     = "issue"
	QueryTypeTimeEntry = "time_entry"
)

// QueryClass maps a saved query kind onto the Redmine query class sent as
// type=. Issue queries map to "", Redmine's default, so their request is
// unchanged.
func QueryClass(kind string) (string, error) {
	switch kind {
	case "", QueryTypeIssue:
		return "", nil
	case QueryTypeTimeEntry:
		return "TimeEntryQuery", nil
	}
	return "", fmt.Errorf("invalid query type %q: use %s or %s", kind, QueryTypeIssue, QueryTypeTimeEntry)
}

type QueriesListResult struct {
	Queries    []models.SavedQuery `json:"queries"`
	Count      int                 `json:"count"`
	TotalCount int                 `json:"total_count"`
}

//mcpgen:tool list_queries
//mcpgen:description List saved queries (custom filters) visible to the authenticated user. Pass the returned id as query_id to list_issues, or to list_time_entries for type time_entry, to run the query.
//mcpgen:category meta
func ListQueries(ctx context.Context, client *api.Client, input ListQueriesInput) (QueriesListResult, error) {
	class, err := QueryClass(input.Type)
	if err != nil {
		return QueriesListResult{}, err
	}
	queries, total, err := client.Queries.ListOfType(ctx, class, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return QueriesListResult{}, err
	}
	return QueriesListResult{Queries: queries, Count: len(queries), TotalCount: total}, nil
}
