// Package mcpserver implements a Model Context Protocol server that exposes
// the redmine-cli API surface to MCP-compatible hosts (Claude Desktop, Claude
// Code, Cursor, etc.).
package mcpserver

// Options controls how the MCP server is built.
type Options struct {
	// EnableWrites gates registration of mutating tools (create/update/delete
	// and similar). When false, those tools are not registered and therefore
	// never appear in tools/list.
	EnableWrites bool

	// Name is the server name advertised in the MCP initialize response.
	Name string

	// Version is the server version advertised in the MCP initialize response.
	Version string

	// EnabledGroups, if non-nil, restricts tool and resource registration to
	// the listed groups. A nil map means "all groups enabled" (the default).
	// An empty non-nil map means "no groups enabled".
	EnabledGroups map[Group]bool

	// EnabledTools, if non-nil, restricts tool registration to the listed
	// tool names within the enabled groups. A nil map means "no allow-list".
	EnabledTools map[string]bool

	// DisabledTools is a deny-list applied after the group and allow-list
	// filters. Names listed here are never registered.
	DisabledTools map[string]bool
}

// groupEnabled reports whether tools or resources belonging to g should be
// registered. A nil EnabledGroups map means all groups are enabled.
func (o Options) groupEnabled(g Group) bool {
	if o.EnabledGroups == nil {
		return true
	}
	return o.EnabledGroups[g]
}

// toolEnabled reports whether a tool with the given name should be registered,
// applying both the optional allow-list and the deny-list. The group filter
// is checked separately by registerToolSpec.
func (o Options) toolEnabled(name string) bool {
	if o.DisabledTools[name] {
		return false
	}
	if o.EnabledTools != nil && !o.EnabledTools[name] {
		return false
	}
	return true
}

// FilterSpec is the user-facing filter input collected from flags, env, and
// config. ResolveOptions merges it into the runtime maps used by Options.
type FilterSpec struct {
	EnableGroups  []Group
	DisableGroups []Group
	EnableTools   []string
	DisableTools  []string
}

// ResolveOptions builds the runtime Options filter maps from a FilterSpec.
//
// Precedence rules:
//   - EnableGroups, when non-empty, narrows the set to only those groups;
//     when empty, all groups start enabled.
//   - DisableGroups subtracts from whatever EnableGroups produced.
//   - EnableTools, when non-empty, becomes the tool allow-list.
//   - DisableTools is the tool deny-list, applied last.
//
// Passing an empty FilterSpec is a no-op, preserving the default behavior of
// "all groups enabled, no per-tool filter".
func ResolveOptions(opts Options, spec FilterSpec) Options {
	if len(spec.EnableGroups) > 0 || len(spec.DisableGroups) > 0 {
		enabled := make(map[Group]bool, len(AllGroups()))
		if len(spec.EnableGroups) == 0 {
			for _, g := range AllGroups() {
				enabled[g] = true
			}
		} else {
			for _, g := range spec.EnableGroups {
				enabled[g] = true
			}
		}
		for _, g := range spec.DisableGroups {
			delete(enabled, g)
		}
		opts.EnabledGroups = enabled
	}

	if len(spec.EnableTools) > 0 {
		allow := make(map[string]bool, len(spec.EnableTools))
		for _, n := range spec.EnableTools {
			allow[n] = true
		}
		opts.EnabledTools = allow
	}
	if len(spec.DisableTools) > 0 {
		deny := make(map[string]bool, len(spec.DisableTools))
		for _, n := range spec.DisableTools {
			deny[n] = true
		}
		opts.DisabledTools = deny
	}
	return opts
}
