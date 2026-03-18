package resolver

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/models"
	lfuzzy "github.com/lithammer/fuzzysearch/fuzzy"
)

// Option represents a name/ID pair for resolution.
type Option struct {
	ID   int
	Name string
}

const (
	smallListThreshold = 10
	maxSuggestions     = 5
)

type suggestion struct {
	name     string
	id       int
	distance int
}

// buildSuggestions generates a helpful error message when no exact match is found.
// For small lists (<=10 items) it shows all options. For larger lists it uses
// Levenshtein distance to suggest close matches.
func buildSuggestions(input string, names []string, ids []int, resourceType string) string {
	if len(names) <= smallListThreshold {
		lines := make([]string, len(names))
		for i := range names {
			lines[i] = fmt.Sprintf("  - %s (ID: %d)", names[i], ids[i])
		}
		return fmt.Sprintf("no match found for %q. Available %ss:\n%s", input, resourceType, strings.Join(lines, "\n"))
	}

	threshold := len(input) / 3
	if threshold < 2 {
		threshold = 2
	}

	var suggestions []suggestion
	lowerInput := strings.ToLower(input)
	for i, name := range names {
		dist := lfuzzy.LevenshteinDistance(lowerInput, strings.ToLower(name))
		if dist <= threshold {
			suggestions = append(suggestions, suggestion{name: name, id: ids[i], distance: dist})
		}
	}

	if len(suggestions) == 0 {
		return fmt.Sprintf("no match found for %q. No similar %ss found.", input, resourceType)
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].distance < suggestions[j].distance
	})

	if len(suggestions) == 1 ||
		(len(suggestions) > 1 && suggestions[0].distance < suggestions[1].distance) {
		return fmt.Sprintf("no match found for %q. Did you mean %q (ID: %d)?", input, suggestions[0].name, suggestions[0].id)
	}

	if len(suggestions) > maxSuggestions {
		suggestions = suggestions[:maxSuggestions]
	}
	lines := make([]string, len(suggestions))
	for i, s := range suggestions {
		lines[i] = fmt.Sprintf("  - %s (ID: %d)", s.name, s.id)
	}
	return fmt.Sprintf("no match found for %q. Did you mean:\n%s", input, strings.Join(lines, "\n"))
}

// Resolve attempts to resolve input to a numeric ID.
// If input is numeric, it returns the parsed int directly.
// Otherwise, it calls fetcher to get available options and performs
// case-insensitive exact matching on the name.
func Resolve(input string, resourceType string, client *api.Client, fetcher func() ([]Option, error)) (int, error) {
	if id, err := strconv.Atoi(input); err == nil {
		client.DebugLog().Printf("Resolver: %s %q is numeric, using ID %d directly", resourceType, input, id)
		return id, nil
	}

	client.DebugLog().Printf("Resolver: looking up %s %q", resourceType, input)

	options, err := fetcher()
	if err != nil {
		return 0, err
	}

	client.DebugLog().Printf("Resolver: searching %d %s options", len(options), resourceType)

	needle := strings.ToLower(input)
	var matches []Option
	for _, o := range options {
		if strings.ToLower(o.Name) == needle {
			matches = append(matches, o)
		}
	}

	switch len(matches) {
	case 0:
		names := make([]string, len(options))
		ids := make([]int, len(options))
		for i, o := range options {
			names[i] = o.Name
			ids[i] = o.ID
		}
		return 0, fmt.Errorf("%s", buildSuggestions(input, names, ids, resourceType))
	case 1:
		client.DebugLog().Printf("Resolver: matched %s %q -> ID %d", resourceType, input, matches[0].ID)
		return matches[0].ID, nil
	default:
		lines := make([]string, len(matches))
		for i, o := range matches {
			lines[i] = fmt.Sprintf("  - %s (ID: %d)", o.Name, o.ID)
		}
		return 0, fmt.Errorf("multiple %ss match %q, please use the numeric ID:\n%s", resourceType, input, strings.Join(lines, "\n"))
	}
}

// ResolveProject resolves a project by name/identifier or numeric ID.
// Returns both the numeric ID and the string identifier (needed for version resolution).
func ResolveProject(ctx context.Context, client *api.Client, input string) (int, string, error) {
	client.DebugLog().Printf("Resolver: resolving project %q", input)
	project, err := client.Projects.Get(ctx, input, nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to resolve project %q: %w", input, err)
	}
	client.DebugLog().Printf("Resolver: resolved project %q -> ID %d, identifier %q", input, project.ID, project.Identifier)
	return project.ID, project.Identifier, nil
}

// ResolveTracker resolves a tracker by name or numeric ID.
func ResolveTracker(ctx context.Context, client *api.Client, input string) (int, error) {
	return Resolve(input, "tracker", client, func() ([]Option, error) {
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
}

// ResolveStatus resolves a status by name or numeric ID.
func ResolveStatus(ctx context.Context, client *api.Client, input string) (int, error) {
	return Resolve(input, "status", client, func() ([]Option, error) {
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
}

// ResolvePriority resolves a priority by name or numeric ID.
func ResolvePriority(ctx context.Context, client *api.Client, input string) (int, error) {
	return Resolve(input, "priority", client, func() ([]Option, error) {
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
}

// ResolveCategory resolves an issue category by name or numeric ID.
// projectIdentifier is required when resolving by name.
func ResolveCategory(ctx context.Context, client *api.Client, input string, projectIdentifier string) (int, error) {
	if id, err := strconv.Atoi(input); err == nil {
		client.DebugLog().Printf("Resolver: category %q is numeric, using ID %d directly", input, id)
		return id, nil
	}

	if projectIdentifier == "" {
		return 0, fmt.Errorf("--project is required when filtering by category name")
	}

	client.DebugLog().Printf("Resolver: looking up category %q in project %q", input, projectIdentifier)

	categories, _, err := client.Categories.List(ctx, projectIdentifier)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch issue categories: %w", err)
	}

	client.DebugLog().Printf("Resolver: searching %d category options", len(categories))

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
		ids := make([]int, len(categories))
		for i, c := range categories {
			names[i] = c.Name
			ids[i] = c.ID
		}
		return 0, fmt.Errorf("%s", buildSuggestions(input, names, ids, "category"))
	case 1:
		client.DebugLog().Printf("Resolver: matched category %q -> ID %d", input, matches[0].ID)
		return matches[0].ID, nil
	default:
		lines := make([]string, len(matches))
		for i, c := range matches {
			lines[i] = fmt.Sprintf("  - %s (ID: %d)", c.Name, c.ID)
		}
		return 0, fmt.Errorf("multiple categories match %q, please use the numeric ID:\n%s", input, strings.Join(lines, "\n"))
	}
}

// ResolveVersion resolves a version by name or numeric ID.
// projectIdentifier is required when resolving by name.
func ResolveVersion(ctx context.Context, client *api.Client, input string, projectIdentifier string) (int, error) {
	if id, err := strconv.Atoi(input); err == nil {
		client.DebugLog().Printf("Resolver: version %q is numeric, using ID %d directly", input, id)
		return id, nil
	}

	if projectIdentifier == "" {
		return 0, fmt.Errorf("--project is required when filtering by version name")
	}

	client.DebugLog().Printf("Resolver: looking up version %q in project %q", input, projectIdentifier)

	versions, _, err := client.Versions.List(ctx, projectIdentifier, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch versions for name resolution: %w", err)
	}

	client.DebugLog().Printf("Resolver: searching %d version options", len(versions))

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
		ids := make([]int, len(versions))
		for i, v := range versions {
			names[i] = v.Name
			ids[i] = v.ID
		}
		return 0, fmt.Errorf("%s", buildSuggestions(input, names, ids, "version"))
	case 1:
		client.DebugLog().Printf("Resolver: matched version %q -> ID %d", input, matches[0].ID)
		return matches[0].ID, nil
	default:
		lines := make([]string, len(matches))
		for i, v := range matches {
			lines[i] = fmt.Sprintf("  - %s (ID: %d)", v.Name, v.ID)
		}
		return 0, fmt.Errorf("multiple versions match %q, please use the numeric ID:\n%s", input, strings.Join(lines, "\n"))
	}
}

// ResolveUser resolves a user by "me", login/name, or numeric ID.
func ResolveUser(ctx context.Context, client *api.Client, input string) (int, error) {
	return resolveUser(ctx, client, input, "user")
}

// ResolveAssignee resolves an assignee by "me", login/name, or numeric ID.
func ResolveAssignee(ctx context.Context, client *api.Client, input string) (int, error) {
	return resolveUser(ctx, client, input, "assignee")
}

func resolveUser(ctx context.Context, client *api.Client, input string, label string) (int, error) {
	if strings.ToLower(input) == "me" {
		client.DebugLog().Printf("Resolver: resolving %s \"me\" via current user", label)
		user, err := client.Users.Current(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get current user: %w", err)
		}
		client.DebugLog().Printf("Resolver: \"me\" -> ID %d (%s %s)", user.ID, user.FirstName, user.LastName)
		return user.ID, nil
	}

	if id, err := strconv.Atoi(input); err == nil {
		client.DebugLog().Printf("Resolver: %s %q is numeric, using ID %d directly", label, input, id)
		return id, nil
	}

	client.DebugLog().Printf("Resolver: looking up %s %q", label, input)

	users, _, err := client.Users.List(ctx, models.UserFilter{Name: input})
	if err != nil {
		return 0, fmt.Errorf("failed to search users: %w", err)
	}

	client.DebugLog().Printf("Resolver: searching %d user results", len(users))

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
		names := make([]string, len(users))
		ids := make([]int, len(users))
		for i, u := range users {
			names[i] = fmt.Sprintf("%s %s / %s", u.FirstName, u.LastName, u.Login)
			ids[i] = u.ID
		}
		if len(names) > 0 {
			return 0, fmt.Errorf("%s", buildSuggestions(input, names, ids, "user"))
		}
		return 0, fmt.Errorf("no user found matching %q", input)
	case 1:
		client.DebugLog().Printf("Resolver: matched %s %q -> ID %d (%s)", label, input, matches[0].ID, matches[0].Login)
		return matches[0].ID, nil
	default:
		lines := make([]string, len(matches))
		for i, u := range matches {
			lines[i] = fmt.Sprintf("  - %s %s / %s (ID: %d)", u.FirstName, u.LastName, u.Login, u.ID)
		}
		return 0, fmt.Errorf("multiple users match %q, please use the numeric ID:\n%s", input, strings.Join(lines, "\n"))
	}
}

// ResolveGroup resolves a group by name or numeric ID.
func ResolveGroup(ctx context.Context, client *api.Client, input string) (int, error) {
	return Resolve(input, "group", client, func() ([]Option, error) {
		groups, _, err := client.Groups.List(ctx, models.GroupFilter{})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch groups: %w", err)
		}
		opts := make([]Option, len(groups))
		for i, g := range groups {
			opts[i] = Option{ID: g.ID, Name: g.Name}
		}
		return opts, nil
	})
}

// ResolveActivity resolves a time entry activity by name or numeric ID.
func ResolveActivity(ctx context.Context, client *api.Client, input string) (int, error) {
	return Resolve(input, "activity", client, func() ([]Option, error) {
		activities, err := client.Enumerations.TimeEntryActivities(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch activities: %w", err)
		}
		opts := make([]Option, len(activities))
		for i, a := range activities {
			opts[i] = Option{ID: a.ID, Name: a.Name}
		}
		return opts, nil
	})
}
