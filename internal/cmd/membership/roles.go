package membership

import (
	"context"
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func resolveRoleIDs(ctx context.Context, client *api.Client, numeric []int, names []string, hasNumeric, hasNames bool) ([]int, error) {
	switch {
	case hasNumeric && hasNames:
		return nil, fmt.Errorf("use either --role-ids or --roles, not both")
	case !hasNumeric && !hasNames:
		return nil, fmt.Errorf("either --role-ids or --roles is required")
	case hasNumeric:
		return numeric, nil
	}

	ids := make([]int, 0, len(names))
	seen := make(map[int]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, err := resolver.ResolveRole(ctx, client, name)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("--roles requires at least one non-empty role name or ID")
	}
	return ids, nil
}
