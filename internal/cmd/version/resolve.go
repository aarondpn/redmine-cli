package version

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func resolveVersionID(ctx context.Context, f *cmdutil.Factory, client *api.Client, arg, project string) (int, error) {
	if id, err := strconv.Atoi(arg); err == nil {
		return id, nil
	}

	project = cmdutil.DefaultProject(f, project)
	if project == "" {
		return 0, fmt.Errorf("--project is required when looking up a version by name")
	}

	project, err := cmdutil.ResolveProjectIdentifier(ctx, f, project)
	if err != nil {
		return 0, err
	}
	return resolver.ResolveVersion(ctx, client, arg, project)
}
