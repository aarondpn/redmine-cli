package resolver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// NoneKeyword is the reserved assignee value meaning "nobody". Commands that
// set an assignee treat it as "clear the assignee"; issue list filters treat
// it as "only unassigned issues".
const NoneKeyword = "none"

// UnassignedFilter is the Redmine issue filter value that matches issues
// without an assignee (assigned_to_id=!*).
const UnassignedFilter = "!*"

// IsNone reports whether input is the "none" keyword (case-insensitive).
func IsNone(input string) bool {
	return strings.EqualFold(strings.TrimSpace(input), NoneKeyword)
}

// NeedsPrincipalLookup reports whether resolving input requires an API
// lookup by name. Empty input, numeric IDs and the "me" / "none" keywords
// are handled without listing users or memberships, so callers can use this
// to skip fetching extra context (such as an issue's project) up front.
func NeedsPrincipalLookup(input string) bool {
	in := strings.TrimSpace(input)
	if in == "" || strings.EqualFold(in, "me") || IsNone(in) {
		return false
	}
	_, err := strconv.Atoi(in)
	return err != nil
}

// ResolveUser resolves a user by "me", login/name, or numeric ID.
func ResolveUser(ctx context.Context, client *api.Client, input string) (int, error) {
	return ResolveUserInProject(ctx, client, input, "")
}

// ResolveAssignee resolves an assignee by "me", login/name, or numeric ID.
// Without project context only the admin-only users endpoint can be used;
// prefer ResolveAssigneeInProject whenever the project is known.
func ResolveAssignee(ctx context.Context, client *api.Client, input string) (int, error) {
	return ResolveAssigneeInProject(ctx, client, input, "")
}

// ResolveAssigneeInProject resolves an assignee ("me", numeric ID, display
// name, or login) in the context of a project. Names are first matched
// against the project's memberships, which list both users and groups and
// are readable by anyone who can see the project, so non-admins can resolve
// names and groups can be assigned. Logins and principals outside the
// project fall back to the admin-only users endpoint. An empty project skips
// the membership lookup.
func ResolveAssigneeInProject(ctx context.Context, client *api.Client, input, project string) (int, error) {
	return newPrincipalLookup(ctx, client, project, "assignee", true).resolve(input)
}

// ResolveUserInProject is ResolveAssigneeInProject restricted to users:
// group memberships are ignored. Use it where Redmine only accepts users,
// such as time entry ownership.
func ResolveUserInProject(ctx context.Context, client *api.Client, input, project string) (int, error) {
	return newPrincipalLookup(ctx, client, project, "user", false).resolve(input)
}

// ResolveAssigneesInProject resolves a batch of principals (users or groups)
// against the same project, fetching the membership list at most once.
// Empty inputs are skipped. label names the inputs in error messages.
func ResolveAssigneesInProject(ctx context.Context, client *api.Client, inputs []string, project, label string) ([]int, error) {
	lookup := newPrincipalLookup(ctx, client, project, label, true)
	ids := make([]int, 0, len(inputs))
	for _, raw := range inputs {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		id, err := lookup.resolve(raw)
		if err != nil {
			return nil, fmt.Errorf("resolving --%s %q: %w", label, raw, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// principal is a user or group that can hold an assignment.
type principal struct {
	ID    int
	Name  string
	Group bool
}

func (p principal) kind() string {
	if p.Group {
		return "group"
	}
	return "user"
}

// principalLookup resolves user/group names, caching the project membership
// list for its own lifetime so batch resolution costs one request.
type principalLookup struct {
	ctx         context.Context
	client      *api.Client
	project     string
	label       string
	allowGroups bool

	membersLoaded bool
	membersOK     bool
	members       []principal
}

func newPrincipalLookup(ctx context.Context, client *api.Client, project, label string, allowGroups bool) *principalLookup {
	return &principalLookup{
		ctx:         ctx,
		client:      client,
		project:     strings.TrimSpace(project),
		label:       label,
		allowGroups: allowGroups,
	}
}

// loadMembers fetches the project's memberships once. Failures (no project
// access, module disabled, ...) are not fatal: they are logged and reported
// as ok=false so the caller falls back to the users endpoint.
func (l *principalLookup) loadMembers() ([]principal, bool) {
	if l.membersLoaded {
		return l.members, l.membersOK
	}
	l.membersLoaded = true

	memberships, _, err := l.client.Memberships.List(l.ctx, l.project, 0, 0)
	if err != nil {
		l.client.DebugLog().Printf("Resolver: listing memberships of project %q failed (%v); falling back to users endpoint", l.project, err)
		return nil, false
	}

	seen := make(map[principal]bool, len(memberships))
	for _, m := range memberships {
		var p principal
		switch {
		case m.User != nil:
			p = principal{ID: m.User.ID, Name: m.User.Name}
		case m.Group != nil && l.allowGroups:
			p = principal{ID: m.Group.ID, Name: m.Group.Name, Group: true}
		default:
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		l.members = append(l.members, p)
	}
	l.membersOK = true
	l.client.DebugLog().Printf("Resolver: loaded %d principals from project %q memberships", len(l.members), l.project)
	return l.members, true
}

func (l *principalLookup) resolve(input string) (int, error) {
	in := strings.TrimSpace(input)

	if strings.EqualFold(in, "me") {
		l.client.DebugLog().Printf("Resolver: resolving %s \"me\" via current user", l.label)
		user, err := l.client.Users.Current(l.ctx, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to get current user: %w", err)
		}
		l.client.DebugLog().Printf("Resolver: \"me\" -> ID %d (%s %s)", user.ID, user.FirstName, user.LastName)
		return user.ID, nil
	}

	if id, err := strconv.Atoi(in); err == nil {
		l.client.DebugLog().Printf("Resolver: %s %q is numeric, using ID %d directly", l.label, in, id)
		return id, nil
	}

	if l.project != "" {
		l.client.DebugLog().Printf("Resolver: looking up %s %q in project %q memberships", l.label, in, l.project)
		if members, ok := l.loadMembers(); ok {
			matches := matchPrincipals(members, in)
			switch len(matches) {
			case 1:
				l.client.DebugLog().Printf("Resolver: matched %s %q -> %s ID %d", l.label, in, matches[0].kind(), matches[0].ID)
				return matches[0].ID, nil
			case 0:
				// Not a member by display name; the input may be a login or a
				// user outside the project, which only the users endpoint knows.
			default:
				lines := make([]string, len(matches))
				for i, p := range matches {
					lines[i] = fmt.Sprintf("  - %s (%s, ID: %d)", p.Name, p.kind(), p.ID)
				}
				return 0, fmt.Errorf("multiple project members match %q, please use the numeric ID:\n%s", in, strings.Join(lines, "\n"))
			}
		}
	}

	return l.resolveViaUsers(in)
}

// resolveViaUsers searches the admin-only /users.json endpoint by login or
// full name. When a membership list is available it is used for the error
// suggestions, since project members are the meaningful candidates.
func (l *principalLookup) resolveViaUsers(in string) (int, error) {
	l.client.DebugLog().Printf("Resolver: looking up %s %q via users endpoint", l.label, in)

	users, _, err := l.client.Users.List(l.ctx, models.UserFilter{Name: in})
	if err != nil {
		if isForbiddenErr(err) {
			if l.membersOK {
				return 0, fmt.Errorf("%s\n%s", l.memberSuggestions(in), "Logins and users outside the project can only be resolved with admin privileges; use the member's name as listed, a numeric ID, or \"me\".")
			}
			return 0, &NameResolutionPermissionError{
				message: fmt.Sprintf("cannot resolve %s by name (listing users requires admin privileges). Use a numeric user ID instead, or \"me\" for yourself", l.label),
			}
		}
		return 0, fmt.Errorf("failed to search users: %w", err)
	}

	l.client.DebugLog().Printf("Resolver: searching %d user results", len(users))

	needle := strings.ToLower(in)
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
		if l.membersOK && len(l.members) > 0 {
			return 0, fmt.Errorf("%s", l.memberSuggestions(in))
		}
		names := make([]string, len(users))
		ids := make([]int, len(users))
		for i, u := range users {
			names[i] = fmt.Sprintf("%s %s / %s", u.FirstName, u.LastName, u.Login)
			ids[i] = u.ID
		}
		if len(names) > 0 {
			return 0, fmt.Errorf("%s", buildSuggestions(in, names, ids, "user"))
		}
		return 0, fmt.Errorf("no user found matching %q", in)
	case 1:
		l.client.DebugLog().Printf("Resolver: matched %s %q -> ID %d (%s)", l.label, in, matches[0].ID, matches[0].Login)
		return matches[0].ID, nil
	default:
		lines := make([]string, len(matches))
		for i, u := range matches {
			lines[i] = fmt.Sprintf("  - %s %s / %s (ID: %d)", u.FirstName, u.LastName, u.Login, u.ID)
		}
		return 0, fmt.Errorf("multiple users match %q, please use the numeric ID:\n%s", in, strings.Join(lines, "\n"))
	}
}

// memberSuggestions renders the "no match" message against the project's
// membership list, labelling each candidate as user or group.
func (l *principalLookup) memberSuggestions(in string) string {
	if len(l.members) == 0 {
		return fmt.Sprintf("no match found for %q: project %q has no members", in, l.project)
	}
	names := make([]string, len(l.members))
	labels := make([]string, len(l.members))
	ids := make([]int, len(l.members))
	for i, p := range l.members {
		names[i] = p.Name
		labels[i] = fmt.Sprintf("%s [%s]", p.Name, p.kind())
		ids[i] = p.ID
	}
	return buildLabeledSuggestions(in, names, labels, ids, "project member")
}

// matchPrincipals returns the principals whose display name equals input,
// case-insensitively.
func matchPrincipals(members []principal, input string) []principal {
	var matches []principal
	for _, p := range members {
		if strings.EqualFold(p.Name, input) {
			matches = append(matches, p)
		}
	}
	return matches
}
