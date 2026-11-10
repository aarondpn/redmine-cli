package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

// The name patterns, as in the struct tags the schema is generated from.
var (
	extensionNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	commandNameRe   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	paramNameRe     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// ReservedExtensionNames can never be extension names: they are the
// extension management command, its alias, and cobra's built-ins. Core
// command names are checked by the caller, which knows the command tree.
var ReservedExtensionNames = []string{"extension", "ext", "help", "completion"}

// ReservedFlagNames are flags a declarative command cannot define: the
// persistent global flags of redmine-cli (a local flag of the same name
// would shadow them), cobra's --help, and the confirmation flags of
// destructive commands (--yes per the extension design, --force as used by
// core delete commands).
var ReservedFlagNames = []string{
	"help", "yes", "force",
	"server", "api-key", "api-key-file", "profile", "config", "header",
	"output", "jq", "no-color", "verbose", "read-only",
}

// ValidateExtensionName checks the extension name rule (pattern, length,
// reserved names). It does not know the core command names.
func ValidateExtensionName(name string) error {
	switch {
	case name == "":
		return errors.New("extension name must not be empty")
	case len(name) > MaxExtensionNameLength:
		return fmt.Errorf("extension name %q is longer than %d characters", name, MaxExtensionNameLength)
	case !extensionNameRe.MatchString(name):
		return fmt.Errorf("extension name %q is invalid: use lowercase letters, digits and dashes, starting with a letter or digit", name)
	case slices.Contains(ReservedExtensionNames, name):
		return fmt.Errorf("extension name %q is reserved", name)
	}
	return nil
}

// validateSemantics records the rules JSON Schema cannot express: unique
// names, reserved names, derived MCP tool names, positional ordering,
// defaults within enum, selectors that compile and terminal-safe texts.
// Rules the schema expresses are enforced by the schema alone (see
// cmd/schemagen), so each problem is reported once. It runs on every
// document that decodes into the structs, including ones the schema
// rejected, and therefore skips values of the wrong shape.
func validateSemantics(is *issues, m *Manifest, opts Options) {
	if slices.Contains(ReservedExtensionNames, m.Name) {
		is.addf("/name", "extension name %q is reserved", m.Name)
	} else if opts.ExtensionName != "" && m.Name != opts.ExtensionName && ValidateExtensionName(m.Name) == nil {
		is.addf("/name", "name %q does not match the extension name %q", m.Name, opts.ExtensionName)
	}
	if m.MinCLIVersion != "" && semverRe.MatchString(m.MinCLIVersion) {
		if _, ok := parseVersion(m.MinCLIVersion); !ok {
			is.addf("/min_cli_version", "%q has a version number that is too large", m.MinCLIVersion)
		}
	}

	validateText(is, m)

	seen := map[string]string{}
	tools := map[string]string{}
	for i := range m.Commands {
		c := &m.Commands[i]
		ptr := pointer("", "commands", i)
		if first, dup := seen[c.Name]; dup && c.Name != "" {
			is.addf(ptr+"/name", "duplicate command name %q (first defined on line %d)", c.Name, is.pos.line(first))
		} else {
			seen[c.Name] = ptr + "/name"
			validateToolName(is, tools, m.Name, c, ptr)
		}
		validateCommand(is, m.Name, c, ptr)
	}
}

// validateText rejects terminal control characters in every string of the
// manifest. Descriptions, enum values and defaults end up in help texts,
// completions and `redmine mcp tools`, which are shown without running the
// extension, so a manifest must not be able to drive the terminal (colors,
// cursor movement, OSC 52 clipboard writes, bidi overrides that reorder
// text). Newlines and tabs are fine.
func validateText(is *issues, m *Manifest) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return
	}
	var walk func(ptr string, v any)
	walk = func(ptr string, v any) {
		switch t := v.(type) {
		case string:
			if r, ok := unsafeRune(t); ok {
				is.addf(ptr, "contains the control character %U; remove it (help texts must not drive the terminal)", r)
			}
		case []any:
			for i, item := range t {
				walk(pointer(ptr, i), item)
			}
		case map[string]any:
			for _, k := range slices.Sorted(maps.Keys(t)) {
				walk(pointer(ptr, k), t[k])
			}
		}
	}
	walk("", doc)
}

// unsafeRune returns the first character of s that textsafe would change
// for the terminal (a C0 or C1 control other than newline and tab, DEL, a
// carriage return, a bidi control, or invalid UTF-8).
func unsafeRune(s string) (rune, bool) {
	if textsafe.IsTerminalSafe(s) {
		return 0, false
	}
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !textsafe.IsTerminalSafe(s[i : i+size]) {
			return r, true
		}
		i += size
	}
	return 0, false
}

// validateToolName rejects two MCP tools of one manifest with the same
// name. Valid command names cannot contain '_', so distinct valid names give
// distinct tools; the check keeps that true if the naming rules or ToolName
// change, and explains why an invalid name such as a_b next to a-b matters.
// tools maps tool names to the first command using them.
func validateToolName(is *issues, tools map[string]string, ext string, c *Command, ptr string) {
	if !c.MCPEnabled() || c.Name == "" {
		return
	}
	tool := ToolName(ext, c.Name)
	if first, dup := tools[tool]; dup {
		is.addf(ptr+"/name", "MCP tool name %q is already used by command %q; rename the command or set mcp: false", tool, first)
		return
	}
	tools[tool] = c.Name
}

func validateCommand(is *issues, ext string, c *Command, ptr string) {
	if c.Description != "" && strings.TrimSpace(c.Description) == "" {
		is.addf(ptr+"/description", "must not be blank")
	}
	if c.Raw {
		return
	}
	if c.MCPEnabled() && commandNameRe.MatchString(c.Name) && ValidateExtensionName(ext) == nil {
		if name := ToolName(ext, c.Name); len(name) > MaxToolNameLength {
			is.addf(ptr+"/name", "MCP tool name %q is longer than %d characters; shorten the extension or command name, or set mcp: false", name, MaxToolNameLength)
		}
	}

	names := map[string]string{}
	for i := range c.Params {
		p := &c.Params[i]
		pp := pointer(ptr, "params", i)
		if first, dup := names[p.Name]; dup && p.Name != "" {
			is.addf(pp+"/name", "duplicate param name %q (first defined on line %d)", p.Name, is.pos.line(first))
		} else {
			names[p.Name] = pp + "/name"
		}
		validateParam(is, p, pp)
	}
	validatePositionals(is, c, ptr)

	if c.Output != nil {
		validateOutput(is, c.Output, ptr+"/output")
	}
}

func validateParam(is *issues, p *Param, ptr string) {
	if !p.Positional && paramNameRe.MatchString(p.Name) {
		if flag := FlagName(p.Name); slices.Contains(ReservedFlagNames, flag) {
			is.addf(ptr+"/name", "flag --%s is reserved by redmine-cli; rename the param or make it positional", flag)
		}
	}
	// The schema checks the type of default and of the enum values; only
	// membership is left, for a default of the right type.
	if p.Default != nil && len(p.Enum) > 0 && (p.Type == TypeString || p.Type == TypeInteger) {
		typed := *p
		typed.Enum = nil
		if checkValue(&typed, p.Default) == nil {
			if err := checkValue(p, p.Default); err != nil {
				is.addf(ptr+"/default", "default %v", err)
			}
		}
	}
}

// validatePositionals enforces the ordering that keeps positional
// arguments unambiguous: required before optional, an array only last.
func validatePositionals(is *issues, c *Command, ptr string) {
	sawOptional := ""
	for i := range c.Params {
		p := &c.Params[i]
		if !p.Positional {
			continue
		}
		pp := pointer(ptr, "params", i)
		if p.Required && sawOptional != "" {
			is.addf(pp+"/required", "required positional %q cannot follow optional positional %q", p.Name, sawOptional)
		}
		if !p.Required {
			sawOptional = p.Name
		}
		if p.Array {
			for j := i + 1; j < len(c.Params); j++ {
				if c.Params[j].Positional {
					is.addf(pp+"/array", "array positional %q must be the last positional", p.Name)
					break
				}
			}
		}
	}
}

func validateOutput(is *issues, o *Output, ptr string) {
	// An empty list is the schema's "must not be empty".
	if o.Table == nil && o.Detail == nil {
		is.addf(ptr, "output must define table or detail")
	}
	for i, col := range o.Table {
		cp := pointer(ptr, "table", i)
		if col.Header != "" && strings.TrimSpace(col.Header) == "" {
			is.addf(cp+"/header", "must not be blank")
		}
		validateSelector(is, col.Path, col.JQ, cp)
	}
	for i, f := range o.Detail {
		fp := pointer(ptr, "detail", i)
		if f.Key != "" && strings.TrimSpace(f.Key) == "" {
			is.addf(fp+"/key", "must not be blank")
		}
		validateSelector(is, f.Path, f.JQ, fp)
	}
}

// validateSelector checks a path or jq selector compiles. That exactly one
// of them is set is a schema rule.
func validateSelector(is *issues, path, jq, ptr string) {
	if jq != "" {
		if _, err := compileJQ(jq); err != nil {
			is.addf(ptr+"/jq", "invalid jq expression: %v", err)
		}
	}
	if path != "" {
		if _, err := splitPath(path); err != nil {
			is.addf(ptr+"/path", "%v", err)
		}
	}
}
