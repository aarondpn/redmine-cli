package category

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// resolveCategoryID turns a category ID or name into an ID. Names are looked
// up in project (--project, falling back to the default project).
func resolveCategoryID(ctx context.Context, f *cmdutil.Factory, client *api.Client, arg, project string) (int, error) {
	if id, err := strconv.Atoi(arg); err == nil {
		return id, nil
	}

	project = cmdutil.DefaultProject(f, project)
	if project == "" {
		return 0, fmt.Errorf("--project is required when looking up a category by name")
	}

	project, err := cmdutil.ResolveProjectIdentifier(ctx, f, project)
	if err != nil {
		return 0, err
	}
	return resolver.ResolveCategory(ctx, client, arg, project)
}

// checkAssigneeFlag validates --assignee before any request is sent: a
// numeric value must be a positive ID, since 0 is how "none" travels
// internally and would silently clear the assignee.
func checkAssigneeFlag(input string) error {
	id, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || id > 0 {
		return nil
	}
	return fmt.Errorf("invalid assignee ID %d: must be a positive user or group ID; use 'none' to clear the default assignee", id)
}
