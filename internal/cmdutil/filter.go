package cmdutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// AddFilterFlag registers the repeatable --filter key=value flag shared by
// the list commands. examples are endpoint-specific key=value samples shown
// in the help text, e.g. "created_on='>=2025-01-01'".
func AddFilterFlag(cmd *cobra.Command, raws *[]string, examples ...string) {
	usage := "Raw Redmine filter as key=value, overriding a flag that sets the same key " +
		"(repeatable; repeat a key ending in [] to send several values)"
	if len(examples) > 0 {
		usage += ". Examples: --filter " + strings.Join(examples, ", --filter ")
	}
	cmd.Flags().StringArrayVar(raws, "filter", nil, usage)
}

// ParseFilterFlag parses repeated --filter key=value entries into raw filter
// parameters. Each entry is split at the first "="; the key is trimmed and
// the value kept verbatim, so "key=" sends an empty value.
//
// A key ending in "[]" (f[], v[status_id][]) collects every value in flag
// order. Any other key keeps its last value, which is also what Redmine would
// use if the key were sent several times.
func ParseFilterFlag(raws []string) (models.FilterParams, error) {
	if len(raws) == 0 {
		return nil, nil
	}
	out := make(models.FilterParams, len(raws))
	for _, raw := range raws {
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--filter %q must be key=value", raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--filter %q: empty key", raw)
		}
		if strings.HasSuffix(key, "[]") {
			out[key] = append(out[key], val)
		} else {
			out[key] = models.FilterValues{val}
		}
	}
	return out, nil
}

// ResolveSavedQueryFlags validates the --query / --query-id flags and returns
// the saved query ID to send as query_id (0 when neither is set). Mutual
// exclusion is enforced by cobra at flag-parse time, so at most one of the
// two is set here. A non-positive --query-id is rejected because Redmine's
// query_id is a positive integer; treating zero as "no filter" would silently
// swallow typos. Names are resolved among the saved queries of queryType
// (e.g. "TimeEntryQuery"; empty means issue queries).
func ResolveSavedQueryFlags(ctx context.Context, client *api.Client, query string, queryID int, queryIDChanged bool, projectIdentifier, queryType string) (int, error) {
	if queryIDChanged {
		if queryID <= 0 {
			return 0, fmt.Errorf("--query-id must be a positive integer, got %d", queryID)
		}
		return queryID, nil
	}
	if query == "" {
		return 0, nil
	}
	q, err := resolver.ResolveQueryOfType(ctx, client, query, projectIdentifier, queryType)
	if err != nil {
		return 0, fmt.Errorf("resolving query: %w", err)
	}
	return q.ID, nil
}
