package api

import (
	"net/url"
	"slices"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// applyFilterParams adds the raw filter parameters to params. It runs after
// the dedicated filters have been set, so a raw key replaces the value a
// dedicated filter set for the same key (e.g. a raw status_id=open|closed
// over the default status_id=open). Every value of a key is sent; a key
// without values is sent once with an empty value.
func applyFilterParams(params url.Values, extra models.FilterParams) {
	for k, vs := range extra {
		if len(vs) == 0 {
			params[k] = []string{""}
			continue
		}
		params[k] = slices.Clone([]string(vs))
	}
}
