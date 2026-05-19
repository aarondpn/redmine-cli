package mcpserver

import (
	"fmt"
	"sort"
	"strings"
)

// Group is the public identifier of a tool group exposed by the MCP server.
// Group names are part of the configuration surface (CLI flags, env vars,
// config file). Renaming a value is a breaking change.
//
// Values mirror the //mcpgen:category directives in internal/ops so the
// generator can stamp each tool with its group at code-generation time.
type Group string

const (
	GroupIssues      Group = "issues"
	GroupProjects    Group = "projects"
	GroupTime        Group = "time"
	GroupUsers       Group = "users"
	GroupMyAccount   Group = "my_account"
	GroupGroups      Group = "groups"
	GroupSearch      Group = "search"
	GroupMeta        Group = "meta"
	GroupWiki        Group = "wiki"
	GroupMemberships Group = "memberships"
)

// AllGroups returns the canonical list of groups in the order used by
// `redmine mcp list-groups` and the documentation.
func AllGroups() []Group {
	return []Group{
		GroupIssues,
		GroupProjects,
		GroupTime,
		GroupUsers,
		GroupMyAccount,
		GroupGroups,
		GroupSearch,
		GroupMeta,
		GroupWiki,
		GroupMemberships,
	}
}

// IsKnownGroup reports whether name matches one of the canonical groups.
func IsKnownGroup(name string) bool {
	for _, g := range AllGroups() {
		if string(g) == name {
			return true
		}
	}
	return false
}

// ParseGroup validates a textual group name and returns the matching Group.
// Names are matched case-insensitively to be friendly to CLI input.
func ParseGroup(name string) (Group, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, g := range AllGroups() {
		if string(g) == want {
			return g, nil
		}
	}
	return "", fmt.Errorf("unknown tool group %q (valid: %s)", name, strings.Join(groupNames(), ", "))
}

// ParseGroups parses a list of group names. Empty entries are skipped so
// callers can pass values like "issues,,wiki" without failing.
func ParseGroups(names []string) ([]Group, error) {
	out := make([]Group, 0, len(names))
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			continue
		}
		g, err := ParseGroup(n)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func groupNames() []string {
	gs := AllGroups()
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = string(g)
	}
	sort.Strings(out)
	return out
}

// ToolDescriptor is a static description of an MCP tool. It is emitted by
// the code generator (zz_generated_tools.go) and consumed by the
// `mcp list-groups` discovery command.
type ToolDescriptor struct {
	Name        string
	Description string
	Group       Group
	Writes      bool
}

// AllTools returns the static catalog of every tool registered by the code
// generator, sorted by group and then name. The result reflects the contents
// of internal/ops/, not the current Options filter.
func AllTools() []ToolDescriptor {
	descriptors := generatedToolDescriptors()
	sort.SliceStable(descriptors, func(i, j int) bool {
		if descriptors[i].Group != descriptors[j].Group {
			return descriptors[i].Group < descriptors[j].Group
		}
		return descriptors[i].Name < descriptors[j].Name
	})
	return descriptors
}

// ToolsByGroup buckets AllTools by group. Groups that are valid but have no
// tools are still included with an empty slice so callers can render every
// group.
func ToolsByGroup() map[Group][]ToolDescriptor {
	out := make(map[Group][]ToolDescriptor, len(AllGroups()))
	for _, g := range AllGroups() {
		out[g] = nil
	}
	for _, d := range AllTools() {
		out[d.Group] = append(out[d.Group], d)
	}
	return out
}

// IsKnownTool reports whether name matches a generated MCP tool name.
func IsKnownTool(name string) bool {
	want := strings.TrimSpace(name)
	for _, d := range AllTools() {
		if d.Name == want {
			return true
		}
	}
	return false
}

// ParseTools validates a list of tool names and returns the trimmed values.
// Empty entries are skipped so callers can pass values like
// "list_issues,,get_issue" without failing.
func ParseTools(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if !IsKnownTool(trimmed) {
			return nil, fmt.Errorf("unknown tool %q", name)
		}
		out = append(out, trimmed)
	}
	return out, nil
}
