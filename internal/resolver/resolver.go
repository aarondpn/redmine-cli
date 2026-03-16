package resolver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/models"
)

// Option represents a name/ID pair for resolution.
type Option struct {
	ID   int
	Name string
}

// Resolve attempts to resolve input to a numeric ID.
// If input is numeric, it returns the parsed int directly.
// Otherwise, it calls fetcher to get available options and performs
// case-insensitive exact matching on the name.
func Resolve(input string, fetcher func() ([]Option, error)) (int, error) {
	if id, err := strconv.Atoi(input); err == nil {
		return id, nil
	}

	options, err := fetcher()
	if err != nil {
		return 0, err
	}

	needle := strings.ToLower(input)
	var matches []Option
	for _, o := range options {
		if strings.ToLower(o.Name) == needle {
			matches = append(matches, o)
		}
	}

	switch len(matches) {
	case 0:
		lines := make([]string, len(options))
		for i, o := range options {
			lines[i] = fmt.Sprintf("  - %s (ID: %d)", o.Name, o.ID)
		}
		return 0, fmt.Errorf("no match found for %q. Available options:\n%s", input, strings.Join(lines, "\n"))
	case 1:
		return matches[0].ID, nil
	default:
		return 0, fmt.Errorf("multiple matches for %q, please use the numeric ID instead", input)
	}
}

// ResolveProject resolves a project by name/identifier or numeric ID.
// Returns both the numeric ID and the string identifier (needed for version resolution).
func ResolveProject(ctx context.Context, client *api.Client, input string) (int, string, error) {
	project, err := client.Projects.Get(ctx, input, nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to resolve project %q: %w", input, err)
	}
	return project.ID, project.Identifier, nil
}

// ResolveTracker resolves a tracker by name or numeric ID.
func ResolveTracker(ctx context.Context, client *api.Client, input string) (int, error) {
	id, err := Resolve(input, func() ([]Option, error) {
		trackers, err := client.Trackers.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch trackers: %w", err)
		}
		opts := make([]Option, len(trackers))
		for i, t := range trackers {
			opts[i] = Option{ID: t.ID, Name: t.Name}
		}
		return opts, nil
	})
	return id, err
}

// ResolveStatus resolves a status by name or numeric ID.
func ResolveStatus(ctx context.Context, client *api.Client, input string) (int, error) {
	id, err := Resolve(input, func() ([]Option, error) {
		statuses, err := client.Statuses.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch statuses: %w", err)
		}
		opts := make([]Option, len(statuses))
		for i, s := range statuses {
			opts[i] = Option{ID: s.ID, Name: s.Name}
		}
		return opts, nil
	})
	return id, err
}

// ResolvePriority resolves a priority by name or numeric ID.
func ResolvePriority(ctx context.Context, client *api.Client, input string) (int, error) {
	id, err := Resolve(input, func() ([]Option, error) {
		priorities, err := client.Enumerations.IssuePriorities(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch priorities: %w", err)
		}
		opts := make([]Option, len(priorities))
		for i, p := range priorities {
			opts[i] = Option{ID: p.ID, Name: p.Name}
		}
		return opts, nil
	})
	return id, err
}

// ResolveCategory resolves an issue category by name or numeric ID.
// projectIdentifier is required when resolving by name.
func ResolveCategory(ctx context.Context, client *api.Client, input string, projectIdentifier string) (int, error) {
	if id, err := strconv.Atoi(input); err == nil {
		return id, nil
	}

	if projectIdentifier == "" {
		return 0, fmt.Errorf("--project is required when filtering by category name")
	}

	categories, _, err := client.Categories.List(ctx, projectIdentifier)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch issue categories: %w", err)
	}

	needle := strings.ToLower(input)
	var matches []models.IssueCategory
	for _, c := range categories {
		if strings.ToLower(c.Name) == needle {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		names := make([]string, len(categories))
		for i, c := range categories {
			names[i] = fmt.Sprintf("  - %s (ID: %d)", c.Name, c.ID)
		}
		return 0, fmt.Errorf("no category found matching %q. Available categories:\n%s", input, strings.Join(names, "\n"))
	case 1:
		return matches[0].ID, nil
	default:
		return 0, fmt.Errorf("multiple categories match %q, please use the category ID instead", input)
	}
}

// ResolveVersion resolves a version by name or numeric ID.
// projectIdentifier is required when resolving by name.
func ResolveVersion(ctx context.Context, client *api.Client, input string, projectIdentifier string) (int, error) {
	if id, err := strconv.Atoi(input); err == nil {
		return id, nil
	}

	if projectIdentifier == "" {
		return 0, fmt.Errorf("--project is required when filtering by version name")
	}

	versions, _, err := client.Versions.List(ctx, projectIdentifier, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch versions for name resolution: %w", err)
	}

	needle := strings.ToLower(input)
	var matches []models.Version
	for _, v := range versions {
		if strings.ToLower(v.Name) == needle {
			matches = append(matches, v)
		}
	}

	switch len(matches) {
	case 0:
		names := make([]string, len(versions))
		for i, v := range versions {
			names[i] = fmt.Sprintf("  - %s (ID: %d)", v.Name, v.ID)
		}
		return 0, fmt.Errorf("no version found matching %q. Available versions:\n%s", input, strings.Join(names, "\n"))
	case 1:
		return matches[0].ID, nil
	default:
		return 0, fmt.Errorf("multiple versions match %q, please use the version ID instead", input)
	}
}

// ResolveAssignee resolves an assignee by "me", login/name, or numeric ID.
func ResolveAssignee(ctx context.Context, client *api.Client, input string) (int, error) {
	if strings.ToLower(input) == "me" {
		user, err := client.Users.Current(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get current user: %w", err)
		}
		return user.ID, nil
	}

	if id, err := strconv.Atoi(input); err == nil {
		return id, nil
	}

	users, _, err := client.Users.List(ctx, models.UserFilter{Name: input})
	if err != nil {
		return 0, fmt.Errorf("failed to search users: %w", err)
	}

	needle := strings.ToLower(input)
	var matches []models.User
	for _, u := range users {
		fullName := strings.ToLower(u.FirstName + " " + u.LastName)
		login := strings.ToLower(u.Login)
		if fullName == needle || login == needle {
			matches = append(matches, u)
		}
	}

	switch len(matches) {
	case 0:
		lines := make([]string, len(users))
		for i, u := range users {
			lines[i] = fmt.Sprintf("  - %s %s / %s (ID: %d)", u.FirstName, u.LastName, u.Login, u.ID)
		}
		if len(lines) > 0 {
			return 0, fmt.Errorf("no exact match for %q. Similar users:\n%s", input, strings.Join(lines, "\n"))
		}
		return 0, fmt.Errorf("no user found matching %q", input)
	case 1:
		return matches[0].ID, nil
	default:
		lines := make([]string, len(matches))
		for i, u := range matches {
			lines[i] = fmt.Sprintf("  - %s %s / %s (ID: %d)", u.FirstName, u.LastName, u.Login, u.ID)
		}
		return 0, fmt.Errorf("multiple users match %q, please use the numeric ID:\n%s", input, strings.Join(lines, "\n"))
	}
}
