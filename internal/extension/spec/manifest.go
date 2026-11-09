// Package spec is the single source of truth for redmine-extension.yml: the
// manifest model, YAML parsing with line positions, the published JSON
// Schema (generated from the structs below), semantic validation, and the
// pure helpers that turn a command definition into a cobra command, an MCP
// tool input schema and rendered output. Process execution lives elsewhere;
// everything here is side-effect free apart from reading @file arguments.
//
// The JSON Schema is generated from the struct tags in this file:
//
//   - json/yaml: property names (identical); omitempty marks optional fields
//   - jsonschema: the property description
//   - jsonschema_extras: further keywords as key=value pairs separated by
//     ';'. List keywords (enum, examples, itemTypes) separate values with
//     '|'. Values are parsed as JSON when possible (so 1 and true keep their
//     type) and taken as strings otherwise.
package spec

//go:generate go run ../../../cmd/schemagen

import "time"

const (
	// FileName is the manifest file name at the root of an extension.
	FileName = "redmine-extension.yml"

	// SchemaID is the published location of the manifest JSON Schema.
	SchemaID = "https://redmine-cli.dev/schema/redmine-extension.v1.json"

	// SchemaFileName is the file name of the generated schema, both next to
	// this package (embedded) and under docs/public/schema (published).
	SchemaFileName = "redmine-extension.v1.json"

	// CurrentSchemaVersion is the only schema_version this build supports.
	CurrentSchemaVersion = 1

	// DefaultTimeout applies when a command does not set timeout.
	DefaultTimeout = 60 * time.Second

	// MaxTimeout caps timeout so a hung extension cannot block an MCP
	// client or a terminal indefinitely.
	MaxTimeout = 5 * time.Minute

	// MaxToolNameLength is the longest MCP tool name clients accept.
	MaxToolNameLength = 64

	// MaxExtensionNameLength bounds extension names so derived names
	// (redmine-cli-<name>, MCP tool names, group ext-<name>) stay short.
	MaxExtensionNameLength = 32
)

// ParamType is the value type of a command parameter.
type ParamType string

const (
	TypeString  ParamType = "string"
	TypeInteger ParamType = "integer"
	TypeNumber  ParamType = "number"
	TypeBoolean ParamType = "boolean"
	// TypeJSON accepts any JSON value: a JSON string or @file on the CLI,
	// any value over MCP.
	TypeJSON ParamType = "json"
)

// ParamTypes lists every supported parameter type in documentation order.
var ParamTypes = []ParamType{TypeString, TypeInteger, TypeNumber, TypeBoolean, TypeJSON}

// Manifest is the content of redmine-extension.yml.
type Manifest struct {
	SchemaVersion int       `yaml:"schema_version" json:"schema_version" jsonschema:"Manifest format version. Only 1 is supported." jsonschema_extras:"enum=1"`
	Name          string    `yaml:"name" json:"name" jsonschema:"Extension name. Must equal <name> in the repository and executable name redmine-cli-<name>; the extension is invoked as 'redmine <name>'." jsonschema_extras:"pattern=^[a-z0-9][a-z0-9-]*$;patternErrorMessage=Use lowercase letters, digits and dashes, starting with a letter or digit.;maxLength=32;examples=pr|deploy"`
	Description   string    `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"Short description shown in 'redmine --help' and 'redmine extension list'." jsonschema_extras:"examples=Work with linked pull requests"`
	MinCLIVersion string    `yaml:"min_cli_version,omitempty" json:"min_cli_version,omitempty" jsonschema:"Lowest redmine-cli version this extension works with (semantic version). Development builds skip the check." jsonschema_extras:"pattern=^v?[0-9]+\\.[0-9]+\\.[0-9]+([-+].*)?$;patternErrorMessage=Use a semantic version such as 2.9.0.;examples=2.9.0"`
	Commands      []Command `yaml:"commands" json:"commands" jsonschema:"Subcommands, invoked as 'redmine <name> <command>'." jsonschema_extras:"minItems=1"`
}

// Command is one subcommand of an extension. A declarative command (the
// default) receives its validated input as JSON on stdin and becomes a CLI
// command and, unless mcp is false, an MCP tool. A raw command is a plain
// passthrough of argv and stdio, CLI only.
type Command struct {
	Name        string  `yaml:"name" json:"name" jsonschema:"Command name, invoked as 'redmine <extension> <name>'. The MCP tool is named <extension>_<name> with dashes replaced by underscores." jsonschema_extras:"pattern=^[a-z][a-z0-9_-]*$;patternErrorMessage=Use lowercase letters, digits, '_' and '-', starting with a letter.;examples=list|create"`
	Description string  `yaml:"description" json:"description" jsonschema:"One-line description used for CLI help and as the MCP tool description." jsonschema_extras:"minLength=1"`
	Writes      bool    `yaml:"writes,omitempty" json:"writes,omitempty" jsonschema:"Whether the command modifies Redmine data. Write tools are only exposed over MCP with --enable-writes; read commands run with REDMINE_READ_ONLY=1." jsonschema_extras:"default=false"`
	Destructive bool    `yaml:"destructive,omitempty" json:"destructive,omitempty" jsonschema:"Whether the command deletes or irreversibly changes data. Adds --yes and a confirmation prompt to the CLI command and marks the MCP tool as destructive. Requires writes: true." jsonschema_extras:"default=false"`
	MCP         *bool   `yaml:"mcp,omitempty" json:"mcp,omitempty" jsonschema:"Whether the command is also exposed as an MCP tool. Set to false for CLI-only commands." jsonschema_extras:"default=true"`
	Raw         bool    `yaml:"raw,omitempty" json:"raw,omitempty" jsonschema:"Pass argv and stdio straight through to the executable (gh-style). Raw commands are CLI only and take no params or output." jsonschema_extras:"default=false"`
	Timeout     string  `yaml:"timeout,omitempty" json:"timeout,omitempty" jsonschema:"Maximum run time as a Go duration (e.g. 30s, 2m). At most 5m." jsonschema_extras:"pattern=^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$;patternErrorMessage=Use a duration such as 30s or 2m.;default=60s;examples=30s|2m"`
	Params      []Param `yaml:"params,omitempty" json:"params,omitempty" jsonschema:"Input parameters. They become CLI flags or positional arguments and the properties of the MCP tool input schema."`
	Output      *Output `yaml:"output,omitempty" json:"output,omitempty" jsonschema:"How the JSON result is rendered in the CLI table format. JSON output and MCP results are never affected."`
}

// Param is one input parameter of a command.
type Param struct {
	Name        string    `yaml:"name" json:"name" jsonschema:"Parameter name and key in the input object. The CLI flag is the name with '_' replaced by '-'." jsonschema_extras:"pattern=^[a-z][a-z0-9_-]*$;patternErrorMessage=Use lowercase letters, digits, '_' and '-', starting with a letter.;examples=issue_id|state"`
	Type        ParamType `yaml:"type" json:"type" jsonschema:"Value type. json accepts a JSON string or @file on the CLI and any JSON value over MCP." jsonschema_extras:"enum=string|integer|number|boolean|json"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty" jsonschema:"Help text for the flag and the MCP tool parameter."`
	Required    bool      `yaml:"required,omitempty" json:"required,omitempty" jsonschema:"Whether the parameter must be given." jsonschema_extras:"default=false"`
	Positional  bool      `yaml:"positional,omitempty" json:"positional,omitempty" jsonschema:"Take the value from a positional argument instead of a flag. Positionals are ordered as declared; required ones come first and an array positional may only be last." jsonschema_extras:"default=false"`
	Array       bool      `yaml:"array,omitempty" json:"array,omitempty" jsonschema:"Accept a list of values (a repeatable flag on the CLI)." jsonschema_extras:"default=false"`
	Enum        []any     `yaml:"enum,omitempty" json:"enum,omitempty" jsonschema:"Allowed values. Only for string and integer parameters." jsonschema_extras:"minItems=1;itemTypes=string|integer"`
	Default     any       `yaml:"default,omitempty" json:"default,omitempty" jsonschema:"Value used when the parameter is not given. Must match type (a list when array is true) and enum."`
}

// Output describes how the CLI table format renders a result. table applies
// to array results, detail to object results; anything else (or a missing
// spec) is printed as pretty JSON.
type Output struct {
	Table  []Column      `yaml:"table,omitempty" json:"table,omitempty" jsonschema:"Columns for array results, one row per element." jsonschema_extras:"minItems=1"`
	Detail []DetailField `yaml:"detail,omitempty" json:"detail,omitempty" jsonschema:"Key/value lines for object results." jsonschema_extras:"minItems=1"`
}

// Column is one table column. Exactly one of Path and JQ is set.
type Column struct {
	Header string `yaml:"header" json:"header" jsonschema:"Column header." jsonschema_extras:"minLength=1;examples=ID|Title"`
	Path   string `yaml:"path,omitempty" json:"path,omitempty" jsonschema:"Dot path into each element (e.g. author.login; numeric segments index arrays)." jsonschema_extras:"minLength=1;examples=id|author.login"`
	JQ     string `yaml:"jq,omitempty" json:"jq,omitempty" jsonschema:"jq expression evaluated against each element. Multiple results are joined with ', '." jsonschema_extras:"minLength=1;examples=.author.login"`
}

// DetailField is one key/value line of a detail view. Exactly one of Path
// and JQ is set.
type DetailField struct {
	Key  string `yaml:"key" json:"key" jsonschema:"Label shown in front of the value." jsonschema_extras:"minLength=1;examples=State"`
	Path string `yaml:"path,omitempty" json:"path,omitempty" jsonschema:"Dot path into the result object (e.g. author.login; numeric segments index arrays)." jsonschema_extras:"minLength=1;examples=state"`
	JQ   string `yaml:"jq,omitempty" json:"jq,omitempty" jsonschema:"jq expression evaluated against the result object. Multiple results are joined with ', '." jsonschema_extras:"minLength=1;examples=.author.login"`
}

// Command returns the command with the given name, or nil.
func (m *Manifest) Command(name string) *Command {
	for i := range m.Commands {
		if m.Commands[i].Name == name {
			return &m.Commands[i]
		}
	}
	return nil
}

// MCPCommands returns the commands that become MCP tools, in manifest order.
func (m *Manifest) MCPCommands() []*Command {
	var out []*Command
	for i := range m.Commands {
		if m.Commands[i].MCPEnabled() {
			out = append(out, &m.Commands[i])
		}
	}
	return out
}

// MCPEnabled reports whether the command is exposed as an MCP tool: raw
// commands never are, declarative ones unless mcp is false.
func (c *Command) MCPEnabled() bool {
	if c.Raw {
		return false
	}
	return c.MCP == nil || *c.MCP
}

// TimeoutDuration returns the effective run time limit. An unset or
// unparsable value (which validation rejects) yields DefaultTimeout.
func (c *Command) TimeoutDuration() time.Duration {
	if c.Timeout == "" {
		return DefaultTimeout
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil || d <= 0 {
		return DefaultTimeout
	}
	return min(d, MaxTimeout)
}

// Positionals returns the positional params in declaration order.
func (c *Command) Positionals() []*Param {
	var out []*Param
	for i := range c.Params {
		if c.Params[i].Positional {
			out = append(out, &c.Params[i])
		}
	}
	return out
}

// FlagParams returns the params that are CLI flags, in declaration order.
func (c *Command) FlagParams() []*Param {
	var out []*Param
	for i := range c.Params {
		if !c.Params[i].Positional {
			out = append(out, &c.Params[i])
		}
	}
	return out
}

// Param returns the param with the given name, or nil.
func (c *Command) Param(name string) *Param {
	for i := range c.Params {
		if c.Params[i].Name == name {
			return &c.Params[i]
		}
	}
	return nil
}
