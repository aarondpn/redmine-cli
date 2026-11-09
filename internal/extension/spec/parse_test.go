package spec

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseExampleManifest(t *testing.T) {
	m, err := ParseFile(filepath.Join("testdata", FileName), Options{ExtensionName: "pr", CLIVersion: "2.9.0"})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if m.SchemaVersion != 1 || m.Name != "pr" || m.MinCLIVersion != "2.9.0" {
		t.Errorf("header = %+v", m)
	}
	if len(m.Commands) != 4 {
		t.Fatalf("got %d commands, want 4", len(m.Commands))
	}

	list := m.Command("list")
	if list == nil {
		t.Fatal("Command(list) = nil")
	}
	if list.TimeoutDuration() != 30*time.Second {
		t.Errorf("list timeout = %v", list.TimeoutDuration())
	}
	if !list.MCPEnabled() || list.Writes || list.Destructive || list.Raw {
		t.Errorf("list flags = %+v", list)
	}
	if got := list.Param("state"); got == nil || got.Default != "open" || !reflect.DeepEqual(got.Enum, []any{"open", "merged", "closed"}) {
		t.Errorf("state param = %+v", got)
	}
	if list.Output == nil || len(list.Output.Table) != 3 || list.Output.Table[2].JQ != ".author.login" || len(list.Output.Detail) != 1 {
		t.Errorf("list output = %+v", list.Output)
	}

	link := m.Command("link-pr")
	wantDefaults := map[string]any{
		"limit": int64(10),
		"ratio": 0.5,
		"draft": true,
		"ids":   []any{int64(1), int64(2)},
		"meta":  map[string]any{"source": "cli"},
	}
	for name, want := range wantDefaults {
		if got := link.Param(name).Default; !reflect.DeepEqual(got, want) {
			t.Errorf("link-pr %s default = %#v, want %#v", name, got, want)
		}
	}
	if got := link.Param("limit").Enum; !reflect.DeepEqual(got, []any{int64(10), int64(20), int64(50)}) {
		t.Errorf("limit enum = %#v", got)
	}
	if link.TimeoutDuration() != DefaultTimeout {
		t.Errorf("link-pr timeout = %v", link.TimeoutDuration())
	}

	unlink := m.Command("unlink")
	if unlink.MCPEnabled() || !unlink.Destructive {
		t.Errorf("unlink = %+v", unlink)
	}
	shell := m.Command("shell")
	if !shell.Raw || shell.MCPEnabled() {
		t.Errorf("shell = %+v", shell)
	}

	var names []string
	for _, c := range m.MCPCommands() {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, []string{"list", "link-pr"}) {
		t.Errorf("MCPCommands = %v", names)
	}
	if m.Command("missing") != nil {
		t.Error("Command(missing) should be nil")
	}
}

const header = "schema_version: 1\nname: pr\n"

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
		want []string // exact error lines, in order
	}{
		{
			name: "yaml syntax",
			src:  header + "commands:\n  - name: [\n",
			want: []string{"redmine-extension.yml:4: did not find expected node content"},
		},
		{
			name: "empty",
			src:  "",
			want: []string{"redmine-extension.yml: manifest is empty"},
		},
		{
			name: "comment only",
			src:  "# nothing\n",
			want: []string{"redmine-extension.yml: manifest is empty"},
		},
		{
			name: "root not a mapping",
			src:  "- a\n- b\n",
			want: []string{"redmine-extension.yml:1: must be an object, got array"},
		},
		{
			name: "duplicate key",
			src:  header + "name: other\ncommands: [{name: a, description: d}]\n",
			want: []string{`redmine-extension.yml:3: duplicate key "name" (first defined on line 2)`},
		},
		{
			name: "merge key",
			src:  header + "base: &b {description: d}\ncommands:\n  - <<: *b\n    name: a\n",
			want: []string{"redmine-extension.yml:5: commands[0]: merge keys (<<) are not supported"},
		},
		{
			name: "non-string key",
			src:  header + "? [a]\n: 1\ncommands: [{name: a, description: d}]\n",
			want: []string{"redmine-extension.yml:3: mapping keys must be strings"},
		},
		{
			name: "missing required top-level fields",
			src:  "description: x\n",
			want: []string{
				`redmine-extension.yml:1: missing required field "schema_version"`,
				`redmine-extension.yml:1: missing required field "name"`,
				`redmine-extension.yml:1: missing required field "commands"`,
			},
		},
		{
			name: "schema_version, name and min version",
			src:  "schema_version: 2\nname: Pr\nmin_cli_version: latest\ncommands: []\n",
			want: []string{
				"redmine-extension.yml:1: schema_version: must be one of: 1 (got 2)",
				`redmine-extension.yml:2: name: "Pr" is invalid: use lowercase letters, digits and dashes, starting with a letter or digit`,
				`redmine-extension.yml:3: min_cli_version: "latest" is invalid: use a semantic version such as 2.9.0`,
				"redmine-extension.yml:4: commands: must not be empty",
			},
		},
		{
			name: "name too long",
			src:  "schema_version: 1\nname: " + strings.Repeat("a", 33) + "\ncommands: [{name: a, description: d}]\n",
			want: []string{"redmine-extension.yml:2: name: must be at most 32 characters (got 33)"},
		},
		{
			name: "wrong types",
			src:  "schema_version: \"1\"\nname: pr\ncommands:\n  - name: a\n    description: d\n    writes: yes please\n    params: {}\n",
			want: []string{
				"redmine-extension.yml:1: schema_version: must be an integer, got string",
				"redmine-extension.yml:6: commands[0].writes: must be a boolean, got string",
				"redmine-extension.yml:7: commands[0].params: must be an array, got object",
			},
		},
		{
			name: "unknown fields at every level",
			src: header + "colour: red\ncommands:\n  - name: a\n    description: d\n    alias: b\n    params:\n" +
				"      - {name: p, type: string, flag: x}\n    output:\n      list: []\n",
			want: []string{
				`redmine-extension.yml:3: unknown field "colour"`,
				`redmine-extension.yml:7: commands[0]: unknown field "alias"`,
				`redmine-extension.yml:9: commands[0].params[0]: unknown field "flag"`,
				`redmine-extension.yml:11: commands[0].output: unknown field "list"`,
			},
		},
		{
			name: "command name, description and timeout",
			src:  header + "commands:\n  - name: List\n    description: \"\"\n    timeout: soon\n",
			want: []string{
				`redmine-extension.yml:4: commands[0].name: "List" is invalid: use lowercase letters, digits, '_' and '-', starting with a letter`,
				"redmine-extension.yml:5: commands[0].description: must not be empty",
				`redmine-extension.yml:6: commands[0].timeout: "soon" is invalid: use a duration such as 30s or 2m`,
			},
		},
		{
			name: "missing command description",
			src:  header + "commands:\n  - name: list\n",
			want: []string{`redmine-extension.yml:4: commands[0]: missing required field "description"`},
		},
		{
			name: "param type and enum conditionals",
			src: header + "commands:\n  - name: a\n    description: d\n    params:\n" +
				"      - name: p\n        type: boolean\n        enum: [a]\n" +
				"      - {name: q, type: strng}\n" +
				"      - {name: r, type: integer, enum: [a, 2]}\n" +
				"      - {name: s}\n",
			want: []string{
				"redmine-extension.yml:9: commands[0].params[0].enum: enum is only allowed for string and integer parameters",
				"redmine-extension.yml:10: commands[0].params[1].type: must be one of: string, integer, number, boolean, json (got strng)",
				"redmine-extension.yml:11: commands[0].params[2].enum: enum values must be integers",
				`redmine-extension.yml:12: commands[0].params[3]: missing required field "type"`,
			},
		},
		{
			name: "default type conditionals",
			src: header + "commands:\n  - name: a\n    description: d\n    params:\n" +
				"      - {name: a, type: integer, default: x}\n" +
				"      - {name: b, type: string, array: true, default: x}\n" +
				"      - {name: c, type: boolean, default: 1}\n" +
				"      - {name: d, type: number, array: true, default: [1, x]}\n" +
				"      - {name: e, type: json, array: true, default: {}}\n" +
				"      - {name: f, type: json, default: {any: thing}}\n" +
				"      - {name: g, type: number, default: 2}\n",
			want: []string{
				"redmine-extension.yml:7: commands[0].params[0].default: default must be an integer",
				"redmine-extension.yml:8: commands[0].params[1].default: default must be a list of strings",
				"redmine-extension.yml:9: commands[0].params[2].default: default must be a boolean",
				"redmine-extension.yml:10: commands[0].params[3].default: default must be a list of numbers",
				"redmine-extension.yml:11: commands[0].params[4].default: default must be a list",
			},
		},
		{
			name: "raw and destructive conditionals",
			src: header + "commands:\n  - name: a\n    description: d\n    raw: true\n    params: [{name: p, type: string}]\n" +
				"  - name: b\n    description: d\n    raw: true\n    mcp: true\n" +
				"  - name: c\n    description: d\n    destructive: true\n",
			want: []string{
				"redmine-extension.yml:4: commands[0]: raw commands take no params or output and are never MCP tools",
				"redmine-extension.yml:8: commands[1]: raw commands take no params or output and are never MCP tools",
				"redmine-extension.yml:12: commands[2]: destructive commands must also set writes: true",
			},
		},
		{
			name: "output selectors",
			src: header + "commands:\n  - name: a\n    description: d\n    output:\n      table:\n" +
				"        - {header: A}\n        - {header: B, path: x, jq: .x}\n        - {path: x}\n        - {header: D, path: \"\"}\n" +
				"      detail: []\n",
			want: []string{
				"redmine-extension.yml:8: commands[0].output.table[0]: set exactly one of path or jq",
				"redmine-extension.yml:9: commands[0].output.table[1]: set exactly one of path or jq",
				`redmine-extension.yml:10: commands[0].output.table[2]: missing required field "header"`,
				"redmine-extension.yml:11: commands[0].output.table[3].path: must not be empty",
				"redmine-extension.yml:12: commands[0].output.detail: must not be empty",
			},
		},
		{
			name: "semantic: duplicates",
			src: header + "commands:\n  - name: a\n    description: d\n    params:\n      - {name: p, type: string}\n      - {name: p, type: integer}\n" +
				"  - name: a\n    description: again\n",
			want: []string{
				`redmine-extension.yml:8: commands[0].params[1].name: duplicate param name "p" (first defined on line 7)`,
				`redmine-extension.yml:9: commands[1].name: duplicate command name "a" (first defined on line 4)`,
			},
		},
		{
			name: "semantic: default outside enum",
			src: header + "commands:\n  - name: a\n    description: d\n    params:\n" +
				"      - {name: s, type: string, enum: [open, closed], default: merged}\n" +
				"      - {name: n, type: integer, array: true, enum: [1, 2], default: [1, 3]}\n",
			want: []string{
				"redmine-extension.yml:7: commands[0].params[0].default: default must be one of: open, closed (got merged)",
				"redmine-extension.yml:8: commands[0].params[1].default: default item 2 must be one of: 1, 2 (got 3)",
			},
		},
		{
			name: "semantic: positional ordering",
			src: header + "commands:\n  - name: a\n    description: d\n    params:\n" +
				"      - {name: opt, type: string, positional: true}\n" +
				"      - {name: req, type: string, positional: true, required: true}\n" +
				"  - name: b\n    description: d\n    params:\n" +
				"      - {name: many, type: string, positional: true, array: true}\n" +
				"      - {name: flag, type: string}\n" +
				"      - {name: last, type: string, positional: true}\n",
			want: []string{
				`redmine-extension.yml:8: commands[0].params[1].required: required positional "req" cannot follow optional positional "opt"`,
				`redmine-extension.yml:12: commands[1].params[0].array: array positional "many" must be the last positional`,
			},
		},
		{
			name: "semantic: flags",
			src: header + "commands:\n  - name: a\n    description: d\n    params:\n" +
				"      - {name: output, type: string}\n" +
				"      - {name: api_key, type: string}\n" +
				"      - {name: output, type: string, positional: true}\n" +
				"      - {name: due_date, type: string}\n" +
				"      - {name: due-date, type: string}\n",
			want: []string{
				"redmine-extension.yml:7: commands[0].params[0].name: flag --output is reserved by redmine-cli; rename the param or make it positional",
				"redmine-extension.yml:8: commands[0].params[1].name: flag --api-key is reserved by redmine-cli; rename the param or make it positional",
				`redmine-extension.yml:9: commands[0].params[2].name: duplicate param name "output" (first defined on line 7)`,
				`redmine-extension.yml:11: commands[0].params[4].name: params "due_date" and "due-date" both map to the flag --due-date`,
			},
		},
		{
			name: "semantic: timeout bounds",
			src:  header + "commands:\n  - name: a\n    description: d\n    timeout: 10m\n  - name: b\n    description: d\n    timeout: 0s\n",
			want: []string{
				"redmine-extension.yml:6: commands[0].timeout: timeout 10m exceeds the maximum of 5m",
				"redmine-extension.yml:9: commands[1].timeout: timeout must be positive",
			},
		},
		{
			name: "semantic: output",
			src: header + "commands:\n  - name: a\n    description: d\n    output: {}\n" +
				"  - name: b\n    description: d\n    output:\n      table:\n        - {header: A, jq: '.a |'}\n        - {header: B, path: a..b}\n",
			want: []string{
				"redmine-extension.yml:6: commands[0].output: output must define table or detail",
				"redmine-extension.yml:11: commands[1].output.table[0].jq: invalid jq expression: unexpected EOF",
				`redmine-extension.yml:12: commands[1].output.table[1].path: path "a..b" has an empty segment`,
			},
		},
		{
			name: "semantic: MCP tool name length",
			src: "schema_version: 1\nname: " + strings.Repeat("e", 32) + "\ncommands:\n" +
				"  - name: " + strings.Repeat("c", 32) + "\n    description: d\n" +
				"  - name: " + strings.Repeat("d", 32) + "\n    description: d\n    mcp: false\n",
			want: []string{
				`redmine-extension.yml:4: commands[0].name: MCP tool name "` + strings.Repeat("e", 32) + "_" + strings.Repeat("c", 32) + `" is longer than 64 characters; shorten the extension or command name, or set mcp: false`,
			},
		},
		{
			name: "semantic: extension name and reserved",
			src:  "schema_version: 1\nname: ext\ncommands: [{name: a, description: d}]\n",
			want: []string{`redmine-extension.yml:2: name: extension name "ext" is reserved`},
		},
		{
			name: "semantic: name mismatch",
			src:  header + "commands: [{name: a, description: d}]\n",
			opts: Options{ExtensionName: "other"},
			want: []string{`redmine-extension.yml:2: name: name "pr" does not match the extension name "other"`},
		},
		{
			name: "semantic: min_cli_version too new",
			src:  header + "min_cli_version: 3.1.0\ncommands: [{name: a, description: d}]\n",
			opts: Options{CLIVersion: "v3.0.9"},
			want: []string{"redmine-extension.yml:3: min_cli_version: requires redmine-cli 3.1.0 or newer (running v3.0.9)"},
		},
		{
			name: "control characters in texts",
			src: "schema_version: 1\nname: pr\ndescription: \"go \\e[31mRED\\e[0m\"\ncommands:\n" +
				"  - name: list\n    description: \"clip \\e]52;c;eA==\\a\"\n    params:\n" +
				"      - { name: state, type: string, enum: [\"a\\x9bb\", ok], description: \"rev\\u202Eersed\" }\n" +
				"    output:\n      table:\n        - { header: \"I\\rD\", path: id }\n",
			want: []string{
				"redmine-extension.yml:3: description: contains the control character U+001B; remove it (help texts must not drive the terminal)",
				"redmine-extension.yml:6: commands[0].description: contains the control character U+001B; remove it (help texts must not drive the terminal)",
				"redmine-extension.yml:8: commands[0].params[0].description: contains the control character U+202E; remove it (help texts must not drive the terminal)",
				"redmine-extension.yml:8: commands[0].params[0].enum[0]: contains the control character U+009B; remove it (help texts must not drive the terminal)",
				"redmine-extension.yml:11: commands[0].output.table[0].header: contains the control character U+000D; remove it (help texts must not drive the terminal)",
			},
		},
		{
			name: "custom file name",
			src:  "schema_version: 2\nname: pr\ncommands: [{name: a, description: d}]\n",
			opts: Options{File: "x/manifest.yml"},
			want: []string{"x/manifest.yml:1: schema_version: must be one of: 1 (got 2)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Parse([]byte(tt.src), tt.opts)
			if err == nil {
				t.Fatalf("Parse succeeded: %+v", m)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("error %T is not *ValidationError: %v", err, err)
			}
			got := ve.Details()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("errors =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(tt.want, "\n  "))
			}
			if err.Error() != strings.Join(tt.want, "\n") {
				t.Errorf("Error() = %q", err.Error())
			}
		})
	}
}

func TestParseAccepts(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts Options
	}{
		{"minimal", header + "commands: [{name: a, description: d}]\n", Options{}},
		{"dev build skips min version", header + "min_cli_version: 99.0.0\ncommands: [{name: a, description: d}]\n", Options{CLIVersion: "dev"}},
		{"git describe version", header + "min_cli_version: 2.9.0\ncommands: [{name: a, description: d}]\n", Options{CLIVersion: "v2.9.0-4-gabc123-dirty"}},
		{"no running version", header + "min_cli_version: 2.9.0\ncommands: [{name: a, description: d}]\n", Options{}},
		{"anchored command", header + "commands:\n  - &a {name: a, description: d}\n  - {name: b, description: d}\n", Options{}},
		{"timestamp-looking string", header + "description: 2024-01-01\ncommands: [{name: a, description: d}]\n", Options{}},
		{"max timeout", header + "commands: [{name: a, description: d, timeout: 5m}]\n", Options{}},
		{"optional positional after required, array last", header +
			"commands:\n  - name: a\n    description: d\n    params:\n" +
			"      - {name: a, type: string, positional: true, required: true}\n" +
			"      - {name: b, type: string, positional: true}\n" +
			"      - {name: c, type: string, positional: true, array: true}\n", Options{}},
		{"reserved flag name as positional", header +
			"commands:\n  - name: a\n    description: d\n    params: [{name: output, type: string, positional: true}]\n", Options{}},
		{"multi-line text with tabs and unicode", header +
			"description: |\n  Größe ✓ 崩溃 🚀\n  \tindented\ncommands: [{name: a, description: \"line one\\nline two\\ttab\"}]\n", Options{}},
		{"exact tool name length", "schema_version: 1\nname: " + strings.Repeat("e", 31) + "\ncommands:\n  - name: " + strings.Repeat("c", 32) + "\n    description: d\n", Options{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.src), tt.opts); err != nil {
				t.Fatalf("Parse: %v", err)
			}
		})
	}
}

func TestParseAliasesExpand(t *testing.T) {
	src := header + "commands:\n  - name: a\n    description: d\n    params: &shared\n      - {name: id, type: integer}\n" +
		"  - name: b\n    description: d\n    params: *shared\n"
	m, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Command("b").Param("id"); got == nil || got.Type != TypeInteger {
		t.Fatalf("alias not expanded: %+v", m.Command("b"))
	}
}

func TestParseAliasBomb(t *testing.T) {
	var b strings.Builder
	b.WriteString(header + "a0: &a0 [x, x, x, x, x, x, x, x, x, x]\n")
	for i := 1; i < 8; i++ {
		b.WriteString("a" + string(rune('0'+i)) + ": &a" + string(rune('0'+i)) + " [")
		for j := 0; j < 10; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("*a" + string(rune('0'+i-1)))
		}
		b.WriteString("]\n")
	}
	_, err := Parse([]byte(b.String()), Options{})
	if err == nil || !strings.Contains(err.Error(), "document is too large") {
		t.Fatalf("Parse = %v, want too-large error", err)
	}
}

func TestParseTooLarge(t *testing.T) {
	data := []byte(header + "description: " + strings.Repeat("x", MaxManifestSize) + "\n")
	_, err := Parse(data, Options{})
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("Parse = %v", err)
	}

	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path, Options{}); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("ParseFile = %v", err)
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := ParseFile(filepath.Join(dir, FileName), Options{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("schema_version: 2\nname: pr\ncommands: [{name: a, description: d}]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path, Options{})
	if err == nil || err.Error() != "redmine-extension.yml:1: schema_version: must be one of: 1 (got 2)" {
		t.Fatalf("ParseFile = %v", err)
	}
}
