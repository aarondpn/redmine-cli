package spec

import (
	"errors"
	"strings"
	"testing"
)

// layer names where a manifest rule is enforced.
type layer string

const (
	// layerSchema rules are in the generated JSON Schema, so editors flag
	// them too. The Go semantic layer must not repeat them.
	layerSchema layer = "schema"
	// layerSemantic rules cannot be expressed in JSON Schema and are only
	// checked by validateSemantics (`redmine extension check`).
	layerSemantic layer = "semantic"
	// layerCompat is the schema_version/min_cli_version gate Parse runs
	// before everything else; it depends on the running redmine-cli.
	layerCompat layer = "compat"
)

// command wraps one command body (indented by four spaces) in a valid
// manifest.
func command(body string) string {
	return header + "commands:\n  - name: a\n    description: d\n" + body
}

// param wraps one param (a flow mapping) in a valid manifest.
func param(p string) string {
	return command("    params:\n      - " + p + "\n")
}

// ruleCorpus is a shared set of manifests with exactly one problem each,
// tagged with the layer that must catch it.
var ruleCorpus = []struct {
	name  string
	src   string
	opts  Options
	layer layer
	want  string // substring of the single error Parse reports
}{
	// Header.
	{"unknown top-level field", header + "colour: red\ncommands: [{name: a, description: d}]\n", Options{}, layerSchema, `unknown field "colour"`},
	{"schema_version wrong type", "schema_version: \"1\"\nname: pr\ncommands: [{name: a, description: d}]\n", Options{}, layerSchema, "must be an integer, got string"},
	{"schema_version newer", "schema_version: 2\nname: pr\ncommands: [{name: a, description: d}]\n", Options{}, layerSchema, "unsupported schema_version 2"},
	{"extension name pattern", "schema_version: 1\nname: Pr\ncommands: [{name: a, description: d}]\n", Options{}, layerSchema, `"Pr" is invalid`},
	{"extension name length", "schema_version: 1\nname: " + strings.Repeat("a", 33) + "\ncommands: [{name: a, description: d}]\n", Options{}, layerSchema, "at most 32 characters"},
	{"min_cli_version format", header + "min_cli_version: latest\ncommands: [{name: a, description: d}]\n", Options{}, layerSchema, "use a semantic version"},
	{"no commands", header + "commands: []\n", Options{}, layerSchema, "must not be empty"},

	// Commands.
	{"command name with underscore", header + "commands: [{name: a_b, description: d}]\n", Options{}, layerSchema, `"a_b" is invalid: use lowercase letters, digits and '-'`},
	{"command name uppercase", header + "commands: [{name: List, description: d}]\n", Options{}, layerSchema, `"List" is invalid`},
	{"command without description", header + "commands: [{name: a}]\n", Options{}, layerSchema, `missing required field "description"`},
	{"timeout zero", command("    timeout: 0s\n"), Options{}, layerSchema, `"0s" is invalid`},
	{"timeout in hours", command("    timeout: 1h\n"), Options{}, layerSchema, `"1h" is invalid`},
	{"timeout fraction", command("    timeout: .5s\n"), Options{}, layerSchema, `".5s" is invalid`},
	{"timeout decimal", command("    timeout: 1.5s\n"), Options{}, layerSchema, `"1.5s" is invalid`},
	{"timeout over 5m in seconds", command("    timeout: 301s\n"), Options{}, layerSchema, `"301s" is invalid`},
	{"timeout over 5m combined", command("    timeout: 5m1s\n"), Options{}, layerSchema, `"5m1s" is invalid`},
	{"timeout over 5m in ms", command("    timeout: 300001ms\n"), Options{}, layerSchema, `"300001ms" is invalid`},
	{"timeout over 5m in minutes", command("    timeout: 10m\n"), Options{}, layerSchema, `"10m" is invalid`},
	{"destructive without writes", command("    destructive: true\n"), Options{}, layerSchema, "destructive commands must also set writes: true"},
	{"raw with params", command("    raw: true\n    params: [{name: p, type: string}]\n"), Options{}, layerSchema, "raw commands take no params"},
	{"raw with output", command("    raw: true\n    output: {table: [{header: A, path: a}]}\n"), Options{}, layerSchema, "raw commands take no output spec"},
	{"raw as MCP tool", command("    raw: true\n    mcp: true\n"), Options{}, layerSchema, "raw commands are never MCP tools"},

	// Params.
	{"param name with dash", param("{name: due-date, type: string}"), Options{}, layerSchema, `"due-date" is invalid: use lowercase letters, digits and '_'`},
	{"param unknown type", param("{name: p, type: date}"), Options{}, layerSchema, "must be one of: string, integer, number, boolean, json (got date)"},
	{"param unknown field", param("{name: p, type: string, flag: x}"), Options{}, layerSchema, `unknown field "flag"`},
	{"enum on number", param("{name: p, type: number, enum: [x]}"), Options{}, layerSchema, "enum is only allowed for string and integer parameters"},
	{"enum value of wrong type", param("{name: p, type: integer, enum: [1, x]}"), Options{}, layerSchema, "enum values must be integers"},
	{"enum empty", param("{name: p, type: string, enum: []}"), Options{}, layerSchema, "must not be empty"},
	{"enum duplicate", param("{name: p, type: string, enum: [a, b, a]}"), Options{}, layerSchema, "must not repeat a value (a is listed twice)"},
	{"enum duplicate integer", param("{name: p, type: integer, enum: [1, 2, 1.0]}"), Options{}, layerSchema, "must not repeat a value (1 is listed twice)"},
	{"default of wrong type", param("{name: p, type: integer, default: x}"), Options{}, layerSchema, "default must be an integer"},
	{"array default not a list", param("{name: p, type: string, array: true, default: x}"), Options{}, layerSchema, "default must be a list of strings"},
	{"boolean array", param("{name: p, type: boolean, array: true}"), Options{}, layerSchema, "boolean parameters cannot be arrays"},
	{"required with default", param("{name: p, type: string, required: true, default: x}"), Options{}, layerSchema, "a required parameter cannot have a default"},

	// Output.
	{"selector with path and jq", command("    output: {table: [{header: A, path: a, jq: .a}]}\n"), Options{}, layerSchema, "set exactly one of path or jq"},
	{"selector missing", command("    output: {table: [{header: A}]}\n"), Options{}, layerSchema, "set exactly one of path or jq"},
	{"column without header", command("    output: {table: [{path: a}]}\n"), Options{}, layerSchema, `missing required field "header"`},
	{"empty detail", command("    output: {detail: []}\n"), Options{}, layerSchema, "must not be empty"},

	// Rules JSON Schema cannot express.
	{"reserved extension name", "schema_version: 1\nname: ext\ncommands: [{name: a, description: d}]\n", Options{}, layerSemantic, `extension name "ext" is reserved`},
	{"extension name mismatch", header + "commands: [{name: a, description: d}]\n", Options{ExtensionName: "other"}, layerSemantic, `does not match the extension name "other"`},
	{"duplicate command", header + "commands: [{name: a, description: d}, {name: a, description: d}]\n", Options{}, layerSemantic, `duplicate command name "a"`},
	{"duplicate param", command("    params: [{name: p, type: string}, {name: p, type: integer}]\n"), Options{}, layerSemantic, `duplicate param name "p"`},
	{"reserved flag", param("{name: output, type: string}"), Options{}, layerSemantic, "flag --output is reserved"},
	{"default outside enum", param("{name: p, type: string, enum: [a], default: b}"), Options{}, layerSemantic, "default must be one of: a (got b)"},
	{"required after optional positional", command("    params:\n      - {name: a, type: string, positional: true}\n      - {name: b, type: string, positional: true, required: true}\n"), Options{}, layerSemantic, `required positional "b" cannot follow optional positional "a"`},
	{"array positional not last", command("    params:\n      - {name: a, type: string, positional: true, array: true}\n      - {name: b, type: string, positional: true}\n"), Options{}, layerSemantic, `array positional "a" must be the last positional`},
	{"tool name too long", "schema_version: 1\nname: " + strings.Repeat("e", 32) + "\ncommands: [{name: " + strings.Repeat("c", 32) + ", description: d}]\n", Options{}, layerSemantic, "is longer than 64 characters"},
	{"output without table or detail", command("    output: {}\n"), Options{}, layerSemantic, "output must define table or detail"},
	{"invalid jq", command("    output: {table: [{header: A, jq: '.a |'}]}\n"), Options{}, layerSemantic, "invalid jq expression"},
	{"invalid path", command("    output: {table: [{header: A, path: a..b}]}\n"), Options{}, layerSemantic, "has an empty segment"},
	{"control character", header + "description: \"a\\e[31m\"\ncommands: [{name: a, description: d}]\n", Options{}, layerSemantic, "control character U+001B"},
	{"blank description", header + "commands: [{name: a, description: \"  \"}]\n", Options{}, layerSemantic, "must not be blank"},

	// Depends on the running redmine-cli.
	{"min_cli_version newer than running", header + "min_cli_version: 9.0.0\ncommands: [{name: a, description: d}]\n", Options{CLIVersion: "2.9.0"}, layerCompat, "requires redmine-cli 9.0.0 or newer (running 2.9.0)"},
}

// validCorpus must pass every layer.
var validCorpus = []string{
	header + "commands: [{name: a, description: d}]\n",
	command("    timeout: 1ms\n"),
	command("    timeout: 500ms\n"),
	command("    timeout: 300000ms\n"),
	command("    timeout: 1s\n"),
	command("    timeout: 300s\n"),
	command("    timeout: 5m\n"),
	command("    timeout: 1m30s\n"),
	command("    timeout: 4m59s\n"),
	command("    writes: true\n    destructive: true\n"),
	command("    raw: true\n    mcp: false\n"),
	header + "commands: [{name: link-pr, description: d}, {name: link, description: d}]\n",
	param("{name: issue_id, type: integer, required: true}"),
	param("{name: draft, type: boolean, default: false}"),
	param("{name: state, type: string, enum: [open, closed], default: open}"),
	param("{name: ids, type: integer, array: true, enum: [1, 2], default: [2]}"),
	param("{name: meta, type: json, array: true, default: [{}, 1]}"),
	param("{name: output, type: string, positional: true}"),
	command("    output: {table: [{header: A, path: a}], detail: [{key: K, jq: .k}]}\n"),
}

// TestRuleLayersAgree runs every corpus manifest through the raw JSON
// Schema (what editors see), the Go semantic layer and Parse. Each rule
// must be enforced by exactly the layer it is tagged with, so editors and
// `redmine extension check` agree on everything the schema can express,
// and Parse must report it exactly once.
func TestRuleLayersAgree(t *testing.T) {
	schema, err := manifestSchema()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.CloneSchemas().Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	layers := func(t *testing.T, src string, opts Options) (schemaRejects, semanticRejects bool) {
		t.Helper()
		doc, pos, err := decodeYAML([]byte(src), FileName)
		if err != nil {
			t.Fatalf("decodeYAML: %v", err)
		}
		schemaRejects = resolved.Validate(doc) != nil
		m, err := fromTree(doc)
		if err != nil {
			// Wrong shapes never reach the semantic layer.
			return schemaRejects, false
		}
		is := &issues{file: FileName, pos: pos}
		validateSemantics(is, m, opts)
		return schemaRejects, len(is.list) > 0
	}

	for _, tt := range ruleCorpus {
		t.Run(tt.name, func(t *testing.T) {
			schemaRejects, semanticRejects := layers(t, tt.src, tt.opts)
			if schemaRejects != (tt.layer == layerSchema) {
				t.Errorf("JSON Schema rejects = %v, want %v (layer %s)", schemaRejects, !schemaRejects, tt.layer)
			}
			if semanticRejects != (tt.layer == layerSemantic) {
				t.Errorf("semantic layer rejects = %v, want %v (layer %s)", semanticRejects, !semanticRejects, tt.layer)
			}
			_, err := Parse([]byte(tt.src), tt.opts)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Parse = %v, want *ValidationError", err)
			}
			if len(ve.Errors) != 1 || !strings.Contains(ve.Errors[0].Error(), tt.want) {
				t.Errorf("Parse errors =\n  %s\nwant one error containing %q", strings.Join(ve.Details(), "\n  "), tt.want)
			}
		})
	}
	for i, src := range validCorpus {
		schemaRejects, semanticRejects := layers(t, src, Options{})
		if schemaRejects || semanticRejects {
			t.Errorf("valid manifest %d rejected (schema %v, semantic %v):\n%s", i, schemaRejects, semanticRejects, src)
		}
		if _, err := Parse([]byte(src), Options{}); err != nil {
			t.Errorf("valid manifest %d: Parse: %v", i, err)
		}
	}
}
