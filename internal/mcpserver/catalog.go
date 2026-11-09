package mcpserver

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// GroupExtensions is the umbrella group of every extension tool. Unlike
	// the other groups it holds no tools of its own: in a filter it stands
	// for all ext-<name> groups, so "--disable-groups extensions" hides
	// every extension at once. It is always valid, also when no extension
	// is installed, so a configuration that rules extensions out keeps
	// working.
	GroupExtensions Group = "extensions"

	// ExtensionGroupPrefix starts the group of an extension's tools,
	// ext-<name>.
	ExtensionGroupPrefix = "ext-"
)

// ExtensionGroup returns the tool group of the extension name.
func ExtensionGroup(name string) Group {
	return Group(ExtensionGroupPrefix + name)
}

// IsExtensionGroup reports whether g is the group of an extension (not the
// umbrella group).
func IsExtensionGroup(g Group) bool {
	return strings.HasPrefix(string(g), ExtensionGroupPrefix)
}

// Catalog is the set of tools and groups a server can expose: the
// generated tools plus the tools of the installed extensions. Filter input
// (--enable-groups, --enable-tools, ...) is validated against it, so a
// group or tool of an installed extension is accepted like a core one.
//
// The package-level AllTools, ToolsByGroup, IsKnownTool, ParseTools and
// ParseGroup(s) describe the generated tools only.
type Catalog struct {
	tools     []ToolDescriptor
	extGroups []Group
}

// NewCatalog combines the generated tools with ext. Extension tools whose
// name is already taken are left out, as ResolveExtensionTools does.
func NewCatalog(ext []ExtensionTool) *Catalog {
	kept, _ := ResolveExtensionTools(ext)
	c := &Catalog{tools: AllTools()}
	seen := map[Group]bool{}
	var extTools []ToolDescriptor
	for _, t := range kept {
		extTools = append(extTools, t.descriptor())
		if g := t.Group(); !seen[g] {
			seen[g] = true
			c.extGroups = append(c.extGroups, g)
		}
	}
	sort.Slice(c.extGroups, func(i, j int) bool { return c.extGroups[i] < c.extGroups[j] })
	sort.SliceStable(extTools, func(i, j int) bool {
		if extTools[i].Group != extTools[j].Group {
			return extTools[i].Group < extTools[j].Group
		}
		return extTools[i].Name < extTools[j].Name
	})
	c.tools = append(c.tools, extTools...)
	return c
}

// Groups returns every group that can be named in a filter: the canonical
// groups (in AllGroups order), the extension groups sorted by name, and
// GroupExtensions last.
func (c *Catalog) Groups() []Group {
	out := AllGroups()
	out = append(out, c.extGroups...)
	return append(out, GroupExtensions)
}

// ExtensionGroups returns the ext-<name> groups of the catalog, sorted.
func (c *Catalog) ExtensionGroups() []Group {
	return append([]Group(nil), c.extGroups...)
}

// AllTools returns every tool of the catalog: the generated ones sorted by
// group and name, then the extension tools sorted the same way.
func (c *Catalog) AllTools() []ToolDescriptor {
	return append([]ToolDescriptor(nil), c.tools...)
}

// ToolsByGroup buckets the catalog's tools by group. Every group of Groups
// is present, empty ones included; GroupExtensions maps to all extension
// tools, the tools it selects.
func (c *Catalog) ToolsByGroup() map[Group][]ToolDescriptor {
	out := make(map[Group][]ToolDescriptor, len(AllGroups())+len(c.extGroups)+1)
	for _, g := range c.Groups() {
		out[g] = nil
	}
	for _, d := range c.tools {
		out[d.Group] = append(out[d.Group], d)
		if d.Extension != "" {
			out[GroupExtensions] = append(out[GroupExtensions], d)
		}
	}
	return out
}

// IsKnownTool reports whether name is a tool of the catalog.
func (c *Catalog) IsKnownTool(name string) bool {
	want := strings.TrimSpace(name)
	for _, d := range c.tools {
		if d.Name == want {
			return true
		}
	}
	return false
}

// IsKnownGroup reports whether name is a group of the catalog.
func (c *Catalog) IsKnownGroup(name string) bool {
	_, err := c.ParseGroup(name)
	return err == nil
}

// ParseGroup validates a group name against the catalog, case-insensitively
// like the package-level ParseGroup.
func (c *Catalog) ParseGroup(name string) (Group, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, g := range c.Groups() {
		if string(g) == want {
			return g, nil
		}
	}
	if IsExtensionGroup(Group(want)) {
		return "", fmt.Errorf("unknown tool group %q: no installed extension %q provides MCP tools (see redmine mcp tools)", name, strings.TrimPrefix(want, ExtensionGroupPrefix))
	}
	names := make([]string, 0, len(AllGroups())+len(c.extGroups)+1)
	for _, g := range c.Groups() {
		names = append(names, string(g))
	}
	sort.Strings(names)
	return "", fmt.Errorf("unknown tool group %q (valid: %s)", name, strings.Join(names, ", "))
}

// ParseGroups parses a list of group names against the catalog. Empty
// entries are skipped.
func (c *Catalog) ParseGroups(names []string) ([]Group, error) {
	out := make([]Group, 0, len(names))
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			continue
		}
		g, err := c.ParseGroup(n)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// ParseTools validates a list of tool names against the catalog and returns
// the trimmed values. Empty entries are skipped.
func (c *Catalog) ParseTools(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if !c.IsKnownTool(trimmed) {
			return nil, fmt.Errorf("unknown tool %q", name)
		}
		out = append(out, trimmed)
	}
	return out, nil
}

// selectableGroups returns the groups a filter starts from when it only
// subtracts: the canonical groups plus the groups of tools. The umbrella
// group is not included, it only expands to the extension groups.
func selectableGroups(tools []ExtensionTool) []Group {
	out := AllGroups()
	seen := map[Group]bool{}
	for _, t := range tools {
		if g := t.Group(); !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}

// expandGroups replaces GroupExtensions in gs with the extension groups of
// tools.
func expandGroups(gs []Group, tools []ExtensionTool) []Group {
	out := make([]Group, 0, len(gs))
	for _, g := range gs {
		if g != GroupExtensions {
			out = append(out, g)
			continue
		}
		for _, sg := range selectableGroups(tools) {
			if IsExtensionGroup(sg) {
				out = append(out, sg)
			}
		}
	}
	return out
}
