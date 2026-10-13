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
// name, or login) in the context of a project. See PrincipalResolver for the
// lookup order. An empty project skips the membership lookup.
func ResolveAssigneeInProject(ctx context.Context, client *api.Client, input, project string) (int, error) {
	return NewPrincipalResolver(ctx, client, project).Assignee(input)
}

// ResolveUserInProject is ResolveAssigneeInProject restricted to users:
// group memberships are ignored. Use it where Redmine only accepts users,
// such as time entry ownership.
func ResolveUserInProject(ctx context.Context, client *api.Client, input, project string) (int, error) {
	return NewPrincipalResolver(ctx, client, project).User(input)
}

// ResolveAssigneesInProject resolves a batch of principals (users or groups)
// against the same project, fetching the membership list at most once.
// Empty inputs are skipped. label names the inputs in error messages.
func ResolveAssigneesInProject(ctx context.Context, client *api.Client, inputs []string, project, label string) ([]int, error) {
	return NewPrincipalResolver(ctx, client, project).Principals(inputs, label)
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

// PrincipalResolver resolves user and group names in the context of one
// project. The project's membership list is fetched at most once and shared
// by every call, so resolving an assignee and several watchers costs a single
// membership request.
//
// Lookup order for a name:
//  1. "me" and numeric IDs never list anything.
//  2. Display names are matched against the project's memberships (users and
//     groups), which any project member can read.
//  3. The admin-only /users.json endpoint is consulted when the input could
//     be a login (no whitespace), when several members share the name, or
//     when no member matched. An exact login match wins over display names,
//     so "max" resolves to the user whose login is max even when another
//     member is displayed as "Max". When the endpoint is forbidden the
//     membership result stands.
type PrincipalResolver struct {
	ctx     context.Context
	client  *api.Client
	project string

	membersLoaded bool
	membersOK     bool
	members       []principal
}

// NewPrincipalResolver returns a resolver bound to project (identifier or
// numeric ID). An empty project disables the membership lookup.
func NewPrincipalResolver(ctx context.Context, client *api.Client, project string) *PrincipalResolver {
	return &PrincipalResolver{ctx: ctx, client: client, project: strings.TrimSpace(project)}
}

// Assignee resolves a user or group that can be assigned an issue.
func (r *PrincipalResolver) Assignee(input string) (int, error) {
	return r.resolve(input, "assignee", true)
}

// User resolves a user; group memberships are ignored.
func (r *PrincipalResolver) User(input string) (int, error) {
	return r.resolve(input, "user", false)
}

// Principals resolves a batch of users or groups (for example watchers).
// Empty inputs are skipped; errors name the offending --label value.
func (r *PrincipalResolver) Principals(inputs []string, label string) ([]int, error) {
	ids := make([]int, 0, len(inputs))
	for _, raw := range inputs {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		id, err := r.resolve(raw, label, true)
		if err != nil {
			return nil, fmt.Errorf("resolving --%s %q: %w", label, raw, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// loadMembers fetches the project's memberships once. Failures (no project
// access, module disabled, ...) are not fatal: they are logged and reported
// as ok=false so the caller falls back to the users endpoint.
func (r *PrincipalResolver) loadMembers() bool {
	if r.membersLoaded {
		return r.membersOK
	}
	r.membersLoaded = true

	memberships, _, err := r.client.Memberships.List(r.ctx, r.project, 0, 0)
	if err != nil {
		r.client.DebugLog().Printf("Resolver: listing memberships of project %q failed (%v); falling back to users endpoint", r.project, err)
		return false
	}

	seen := make(map[principal]bool, len(memberships))
	for _, m := range memberships {
		var p principal
		switch {
		case m.User != nil:
			p = principal{ID: m.User.ID, Name: m.User.Name}
		case m.Group != nil:
			p = principal{ID: m.Group.ID, Name: m.Group.Name, Group: true}
		default:
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		r.members = append(r.members, p)
	}
	r.membersOK = true
	r.client.DebugLog().Printf("Resolver: loaded %d principals from project %q memberships", len(r.members), r.project)
	return true
}

// candidates returns the loaded members, without groups unless allowed.
func (r *PrincipalResolver) candidates(allowGroups bool) []principal {
	if allowGroups {
		return r.members
	}
	users := make([]principal, 0, len(r.members))
	for _, p := range r.members {
		if !p.Group {
			users = append(users, p)
		}
	}
	return users
}

func (r *PrincipalResolver) resolve(input, label string, allowGroups bool) (int, error) {
	in := strings.TrimSpace(input)

	if strings.EqualFold(in, "me") {
		r.client.DebugLog().Printf("Resolver: resolving %s \"me\" via current user", label)
		user, err := r.client.Users.Current(r.ctx, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to get current user: %w", err)
		}
		r.client.DebugLog().Printf("Resolver: \"me\" -> ID %d (%s %s)", user.ID, user.FirstName, user.LastName)
		return user.ID, nil
	}

	if id, err := strconv.Atoi(in); err == nil {
		r.client.DebugLog().Printf("Resolver: %s %q is numeric, using ID %d directly", label, in, id)
		return id, nil
	}

	var memberMatches []principal
	membersOK := false
	if r.project != "" {
		r.client.DebugLog().Printf("Resolver: looking up %s %q in project %q memberships", label, in, r.project)
		if r.loadMembers() {
			membersOK = true
			memberMatches = matchPrincipals(r.candidates(allowGroups), in)
		}
	}

	// A unique display-name match is final unless the input could also be a
	// login, which only the users endpoint can confirm.
	if len(memberMatches) == 1 && !looksLikeLogin(in) {
		r.client.DebugLog().Printf("Resolver: matched %s %q -> %s ID %d", label, in, memberMatches[0].kind(), memberMatches[0].ID)
		return memberMatches[0].ID, nil
	}

	r.client.DebugLog().Printf("Resolver: looking up %s %q via users endpoint", label, in)
	users, _, err := r.client.Users.List(r.ctx, models.UserFilter{Name: in})
	if err != nil {
		if len(memberMatches) > 0 {
			r.client.DebugLog().Printf("Resolver: users endpoint unavailable (%v); using membership match", err)
			return memberResult(in, memberMatches)
		}
		if isForbiddenErr(err) {
			if membersOK {
				if groupErr := r.groupNotAllowed(in, allowGroups); groupErr != nil {
					return 0, groupErr
				}
				return 0, fmt.Errorf("%s\n%s", r.memberSuggestions(in, allowGroups), "Logins and users outside the project can only be resolved with admin privileges; use the member's name as listed, a numeric ID, or \"me\".")
			}
			return 0, &NameResolutionPermissionError{
				message: fmt.Sprintf("cannot resolve %s by name (listing users requires admin privileges). Use a numeric user ID instead, or \"me\" for yourself", label),
			}
		}
		return 0, fmt.Errorf("failed to search users: %w", err)
	}

	r.client.DebugLog().Printf("Resolver: searching %d user results", len(users))

	// An exact login match wins over display names.
	for _, u := range users {
		if strings.EqualFold(u.Login, in) {
			r.client.DebugLog().Printf("Resolver: matched %s %q -> login %s ID %d", label, in, u.Login, u.ID)
			return u.ID, nil
		}
	}

	if len(memberMatches) > 0 {
		return memberResult(in, memberMatches)
	}

	needle := strings.ToLower(in)
	var matches []models.User
	for _, u := range users {
		if strings.ToLower(u.FirstName+" "+u.LastName) == needle {
			matches = append(matches, u)
		}
	}

	switch len(matches) {
	case 0:
		if membersOK {
			if groupErr := r.groupNotAllowed(in, allowGroups); groupErr != nil {
				return 0, groupErr
			}
		}
		if membersOK && len(r.candidates(allowGroups)) > 0 {
			return 0, fmt.Errorf("%s", r.memberSuggestions(in, allowGroups))
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
		r.client.DebugLog().Printf("Resolver: matched %s %q -> ID %d (%s)", label, in, matches[0].ID, matches[0].Login)
		return matches[0].ID, nil
	default:
		lines := make([]string, len(matches))
		for i, u := range matches {
			lines[i] = fmt.Sprintf("  - %s %s / %s (ID: %d)", u.FirstName, u.LastName, u.Login, u.ID)
		}
		return 0, fmt.Errorf("multiple users match %q, please use the numeric ID:\n%s", in, strings.Join(lines, "\n"))
	}
}

// groupNotAllowed explains a user-only lookup that failed because input
// names a group of the project. Without it the caller would get a "no similar
// members" list that leaves out the very group they typed. It returns nil
// when groups are allowed or no group matches.
func (r *PrincipalResolver) groupNotAllowed(in string, allowGroups bool) error {
	if allowGroups {
		return nil
	}
	for _, p := range matchPrincipals(r.members, in) {
		if p.Group {
			return fmt.Errorf("%q is a group (ID: %d), but only users are accepted here: groups cannot be issue authors or own time entries", p.Name, p.ID)
		}
	}
	return nil
}

// memberResult turns display-name matches into an ID, or an ambiguity error
// that lists every matching member when more than one shares the name.
func memberResult(in string, matches []principal) (int, error) {
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	lines := make([]string, len(matches))
	for i, p := range matches {
		lines[i] = fmt.Sprintf("  - %s (%s, ID: %d)", p.Name, p.kind(), p.ID)
	}
	return 0, fmt.Errorf("multiple project members match %q, please use the numeric ID:\n%s", in, strings.Join(lines, "\n"))
}

// memberSuggestions renders the "no match" message against the project's
// membership list, labelling each candidate as user or group.
func (r *PrincipalResolver) memberSuggestions(in string, allowGroups bool) string {
	members := r.candidates(allowGroups)
	if len(members) == 0 {
		return fmt.Sprintf("no match found for %q: project %q has no members", in, r.project)
	}
	names := make([]string, len(members))
	labels := make([]string, len(members))
	ids := make([]int, len(members))
	for i, p := range members {
		names[i] = p.Name
		labels[i] = fmt.Sprintf("%s [%s]", p.Name, p.kind())
		ids[i] = p.ID
	}
	return buildLabeledSuggestions(in, names, labels, ids, "project member")
}

// looksLikeLogin reports whether input could be a Redmine login. Logins
// cannot contain whitespace, so "Jane Doe" is only ever a display name.
func looksLikeLogin(input string) bool {
	return !strings.ContainsAny(input, " \t")
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
