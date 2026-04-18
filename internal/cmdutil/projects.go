package cmdutil

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// ResolveProjectID resolves a project input to its numeric ID string.
func ResolveProjectID(ctx context.Context, f *Factory, input string) (string, error) {
	if input == "" {
		return "", nil
	}

	client, err := f.ApiClient()
	if err != nil {
		return "", err
	}

	id, _, err := resolver.ResolveProject(ctx, client, input)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(id), nil
}

// ResolveProjectIdentifier resolves a project input to its canonical identifier.
func ResolveProjectIdentifier(ctx context.Context, f *Factory, input string) (string, error) {
	if input == "" {
		return "", nil
	}

	client, err := f.ApiClient()
	if err != nil {
		return "", err
	}

	_, identifier, err := resolver.ResolveProject(ctx, client, input)
	if err != nil {
		return "", err
	}

	return identifier, nil
}

// DefaultProjectID applies the default-project fallback and resolves to a
// numeric ID string. Returns ("", nil) when no project is specified.
func DefaultProjectID(ctx context.Context, f *Factory, project string) (string, error) {
	return ResolveProjectID(ctx, f, DefaultProject(f, project))
}

// DefaultProjectIdentifier applies the default-project fallback and resolves
// to the canonical identifier. Returns ("", nil) when no project is specified.
func DefaultProjectIdentifier(ctx context.Context, f *Factory, project string) (string, error) {
	return ResolveProjectIdentifier(ctx, f, DefaultProject(f, project))
}

// RequireProjectIdentifier applies the default-project fallback, ensures a
// project was provided, and resolves to the canonical identifier.
func RequireProjectIdentifier(ctx context.Context, f *Factory, project string) (string, error) {
	project = DefaultProject(f, project)
	if project == "" {
		return "", fmt.Errorf("project is required (use --project or set a default project)")
	}
	return ResolveProjectIdentifier(ctx, f, project)
}
