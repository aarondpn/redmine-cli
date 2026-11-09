package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

var (
	extensionNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	identifierRe    = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
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

// Validate checks a manifest built in code: the same JSON Schema and
// semantic rules Parse applies, without line positions.
func (m *Manifest) Validate(opts Options) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	file := opts.File
	if err := validateAgainstSchema(doc, nil, file); err != nil {
		return err
	}
	return validateSemantics(m, opts, nil)
}

// validateSemantics enforces the rules JSON Schema cannot express (and,
// for manifests built in code, repeats the ones it can).
func validateSemantics(m *Manifest, opts Options, pos positions) error {
	is := &issues{file: opts.File, pos: pos}
	if pos != nil {
		is.file = opts.file()
	}

	if m.SchemaVersion != CurrentSchemaVersion {
		is.addf("/schema_version", "unsupported schema_version %d (this redmine-cli supports %d)", m.SchemaVersion, CurrentSchemaVersion)
	}
	if err := ValidateExtensionName(m.Name); err != nil {
		is.addf("/name", "%v", err)
	} else if opts.ExtensionName != "" && m.Name != opts.ExtensionName {
		is.addf("/name", "name %q does not match the extension name %q", m.Name, opts.ExtensionName)
	}
	if m.MinCLIVersion != "" {
		want, ok := parseVersion(m.MinCLIVersion)
		if !ok {
			is.addf("/min_cli_version", "%q is not a semantic version", m.MinCLIVersion)
		} else if running, release := parseVersion(opts.CLIVersion); release && compareVersions(running, want) < 0 {
			is.addf("/min_cli_version", "requires redmine-cli %s or newer (running %s)", m.MinCLIVersion, opts.CLIVersion)
		}
	}
	if len(m.Commands) == 0 {
		is.addf("/commands", "at least one command is required")
	}

	validateText(is, m)

	seen := map[string]string{}
	for i := range m.Commands {
		c := &m.Commands[i]
		ptr := pointer("", "commands", i)
		if first, dup := seen[c.Name]; dup && c.Name != "" {
			is.addf(ptr+"/name", "duplicate command name %q (first defined on line %d)", c.Name, is.pos.line(first))
		} else {
			seen[c.Name] = ptr + "/name"
		}
		validateCommand(is, m.Name, c, ptr)
	}
	return is.err()
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

func validateCommand(is *issues, ext string, c *Command, ptr string) {
	if !identifierRe.MatchString(c.Name) {
		is.addf(ptr+"/name", "command name %q is invalid: use lowercase letters, digits, '_' and '-', starting with a letter", c.Name)
	}
	if strings.TrimSpace(c.Description) == "" {
		is.addf(ptr, "missing required field %q", "description")
	}
	if c.Timeout != "" {
		d, err := time.ParseDuration(c.Timeout)
		switch {
		case err != nil:
			is.addf(ptr+"/timeout", "%q is not a duration (use e.g. 30s or 2m)", c.Timeout)
		case d <= 0:
			is.addf(ptr+"/timeout", "timeout must be positive")
		case d > MaxTimeout:
			is.addf(ptr+"/timeout", "timeout %s exceeds the maximum of %s", c.Timeout, fmtDuration(MaxTimeout))
		}
	}
	if c.Destructive && !c.Writes {
		is.addf(ptr+"/destructive", "destructive commands must also set writes: true")
	}
	if c.Raw {
		if len(c.Params) > 0 {
			is.addf(ptr+"/params", "raw commands take no params (arguments are passed through)")
		}
		if c.Output != nil {
			is.addf(ptr+"/output", "raw commands take no output spec (stdout is passed through)")
		}
		if c.MCP != nil && *c.MCP {
			is.addf(ptr+"/mcp", "raw commands are never MCP tools")
		}
		return
	}
	if c.MCPEnabled() {
		if name := ToolName(ext, c.Name); len(name) > MaxToolNameLength {
			is.addf(ptr+"/name", "MCP tool name %q is longer than %d characters; shorten the extension or command name, or set mcp: false", name, MaxToolNameLength)
		}
	}

	names := map[string]string{}
	flags := map[string]string{}
	for i := range c.Params {
		p := &c.Params[i]
		pp := pointer(ptr, "params", i)
		if first, dup := names[p.Name]; dup && p.Name != "" {
			is.addf(pp+"/name", "duplicate param name %q (first defined on line %d)", p.Name, is.pos.line(first))
		} else {
			names[p.Name] = pp + "/name"
		}
		validateParam(is, p, pp)
		if p.Positional || !identifierRe.MatchString(p.Name) {
			continue
		}
		flag := FlagName(p.Name)
		if slices.Contains(ReservedFlagNames, flag) {
			is.addf(pp+"/name", "flag --%s is reserved by redmine-cli; rename the param or make it positional", flag)
		} else if other, clash := flags[flag]; clash && other != p.Name {
			is.addf(pp+"/name", "params %q and %q both map to the flag --%s", other, p.Name, flag)
		} else {
			flags[flag] = p.Name
		}
	}
	validatePositionals(is, c, ptr)

	if c.Output != nil {
		validateOutput(is, c.Output, ptr+"/output")
	}
}

func validateParam(is *issues, p *Param, ptr string) {
	if !identifierRe.MatchString(p.Name) {
		is.addf(ptr+"/name", "param name %q is invalid: use lowercase letters, digits, '_' and '-', starting with a letter", p.Name)
	}
	if !slices.Contains(ParamTypes, p.Type) {
		is.addf(ptr+"/type", "unknown type %q (use one of: string, integer, number, boolean, json)", p.Type)
		return
	}
	if p.Enum != nil {
		if p.Type != TypeString && p.Type != TypeInteger {
			is.addf(ptr+"/enum", "enum is only allowed for string and integer parameters")
		} else if len(p.Enum) == 0 {
			is.addf(ptr+"/enum", "enum must not be empty")
		} else {
			for i, v := range p.Enum {
				if _, ok := coerceScalar(p.Type, v); !ok {
					is.addf(pointer(ptr, "enum", i), "enum value %s is not %s", formatValue(v), article(p.Type))
				}
			}
		}
	}
	if p.Default != nil {
		if err := checkValue(p, p.Default); err != nil {
			is.addf(ptr+"/default", "default %v", err)
		}
	}
}

// validatePositionals enforces the ordering that keeps positional
// arguments unambiguous: required before optional, an array only last.
func validatePositionals(is *issues, c *Command, ptr string) {
	positionals := 0
	sawOptional := ""
	for i := range c.Params {
		p := &c.Params[i]
		if !p.Positional {
			continue
		}
		positionals++
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
	if len(o.Table) == 0 && len(o.Detail) == 0 {
		is.addf(ptr, "output must define table or detail")
	}
	for i, col := range o.Table {
		cp := pointer(ptr, "table", i)
		if strings.TrimSpace(col.Header) == "" {
			is.addf(cp, "missing required field %q", "header")
		}
		validateSelector(is, col.Path, col.JQ, cp)
	}
	for i, f := range o.Detail {
		fp := pointer(ptr, "detail", i)
		if strings.TrimSpace(f.Key) == "" {
			is.addf(fp, "missing required field %q", "key")
		}
		validateSelector(is, f.Path, f.JQ, fp)
	}
}

func validateSelector(is *issues, path, jq, ptr string) {
	switch {
	case path != "" && jq != "":
		is.addf(ptr, "set exactly one of path or jq")
	case path == "" && jq == "":
		is.addf(ptr, "set exactly one of path or jq")
	case jq != "":
		if _, err := compileJQ(jq); err != nil {
			is.addf(ptr+"/jq", "invalid jq expression: %v", err)
		}
	default:
		if _, err := splitPath(path); err != nil {
			is.addf(ptr+"/path", "%v", err)
		}
	}
}

// fmtDuration renders 5m0s as 5m.
func fmtDuration(d time.Duration) string {
	s := d.String()
	s = strings.TrimSuffix(s, "0s")
	if strings.HasSuffix(s, "m") && strings.Contains(s, "h") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
