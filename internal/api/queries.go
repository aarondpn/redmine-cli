package api

import (
	"context"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// QueryService handles saved-query API calls.
type QueryService struct {
	client *Client
}

// List retrieves saved queries visible to the authenticated user. Redmine's
// /queries.json endpoint does not accept a project filter; callers receive
// every visible query (global plus project-specific) and can filter by the
// ProjectID field client-side if needed.
func (s *QueryService) List(ctx context.Context, limit, offset int) ([]models.SavedQuery, int, error) {
	params := url.Values{}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.SavedQuery](ctx, s.client, "/queries.json", params, "queries", limit)
}
