package cmdutil

import (
	"context"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/resolver"
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
