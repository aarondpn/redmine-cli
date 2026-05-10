package ops

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListQueriesInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset int `json:"offset,omitempty" jsonschema:"Number of leading results to skip (pagination)."`
}

type QueriesListResult struct {
	Queries    []models.SavedQuery `json:"queries"`
	Count      int                 `json:"count"`
	TotalCount int                 `json:"total_count"`
}

//mcpgen:tool list_queries
//mcpgen:description List saved queries (custom filters) visible to the authenticated user. Use the returned id with list_issues' query_id parameter.
//mcpgen:category meta
func ListQueries(ctx context.Context, client *api.Client, input ListQueriesInput) (QueriesListResult, error) {
	queries, total, err := client.Queries.List(ctx, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return QueriesListResult{}, err
	}
	return QueriesListResult{Queries: queries, Count: len(queries), TotalCount: total}, nil
}
