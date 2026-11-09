package spec

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func boolPtr(b bool) *bool { return &b }

func validManifest() *Manifest {
	return &Manifest{
		SchemaVersion: 1,
		Name:          "pr",
		Commands: []Command{{
			Name:        "list",
			Description: "List",
			Params: []Param{
				{Name: "issue_id", Type: TypeInteger, Required: true, Positional: true},
				{Name: "state", Type: TypeString, Enum: []any{"open", "closed"}, Default: "open"},
			},
		}},
	}
}

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m *Manifest)
		opts   Options
		want   []string
	}{
		{name: "valid", mutate: func(*Manifest) {}},
		{
			name:   "schema rules apply to code-built manifests",
			mutate: func(m *Manifest) { m.Commands[0].Params[0].Type = "date" },
			want:   []string{"commands[0].params[0].type: must be one of: string, integer, number, boolean, json (got date)"},
		},
		{
			name: "raw command with params",
			mutate: func(m *Manifest) {
				m.Commands[0].Raw = true
				m.Commands[0].MCP = boolPtr(true)
			},
			want: []string{"commands[0]: raw commands take no params or output and are never MCP tools"},
		},
		{
			name:   "default outside enum",
			mutate: func(m *Manifest) { m.Commands[0].Params[1].Default = "merged" },
			want:   []string{"commands[0].params[1].default: default must be one of: open, closed (got merged)"},
		},
		{
			name:   "file name prefix",
			mutate: func(m *Manifest) { m.Commands[0].Params[1].Default = "merged" },
			opts:   Options{File: "redmine-extension.yml"},
			want:   []string{"redmine-extension.yml: commands[0].params[1].default: default must be one of: open, closed (got merged)"},
		},
		{
			name:   "extension name mismatch",
			mutate: func(*Manifest) {},
			opts:   Options{ExtensionName: "issues"},
			want:   []string{`name: name "pr" does not match the extension name "issues"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.mutate(m)
			err := m.Validate(tt.opts)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Validate = %v, want *ValidationError", err)
			}
			if got := ve.Details(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("errors =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(tt.want, "\n  "))
			}
		})
	}
}

// TestValidateSemanticsDirect covers the Go-side copies of rules the schema
// also enforces: they matter for manifests whose schema check is skipped.
func TestValidateSemanticsDirect(t *testing.T) {
	tests := []struct {
		name string
		m    Manifest
		want []string
	}{
		{
			name: "header",
			m:    Manifest{SchemaVersion: 2, Name: "Bad Name", MinCLIVersion: "two"},
			want: []string{
				"schema_version: unsupported schema_version 2 (this redmine-cli supports 1)",
				`name: extension name "Bad Name" is invalid: use lowercase letters, digits and dashes, starting with a letter or digit`,
				`min_cli_version: "two" is not a semantic version`,
				"commands: at least one command is required",
			},
		},
		{
			name: "command rules",
			m: Manifest{SchemaVersion: 1, Name: "pr", Commands: []Command{
				{Name: "Bad", Description: " ", Timeout: "x", Destructive: true},
				{Name: "raw", Description: "d", Raw: true, MCP: boolPtr(true), Params: []Param{{Name: "a", Type: TypeString}}, Output: &Output{}},
			}},
			want: []string{
				`commands[0].name: command name "Bad" is invalid: use lowercase letters, digits, '_' and '-', starting with a letter`,
				`commands[0]: missing required field "description"`,
				`commands[0].timeout: "x" is not a duration (use e.g. 30s or 2m)`,
				"commands[0].destructive: destructive commands must also set writes: true",
				"commands[1].params: raw commands take no params (arguments are passed through)",
				"commands[1].output: raw commands take no output spec (stdout is passed through)",
				"commands[1].mcp: raw commands are never MCP tools",
			},
		},
		{
			name: "param rules",
			m: Manifest{SchemaVersion: 1, Name: "pr", Commands: []Command{{Name: "a", Description: "d", Params: []Param{
				{Name: "Bad", Type: TypeString},
				{Name: "t", Type: "date"},
				{Name: "b", Type: TypeBoolean, Enum: []any{true}},
				{Name: "e", Type: TypeString, Enum: []any{}},
				{Name: "i", Type: TypeInteger, Enum: []any{"x", int64(1)}},
				{Name: "j", Type: TypeJSON, Array: true, Default: "x"},
			}}}},
			want: []string{
				`commands[0].params[0].name: param name "Bad" is invalid: use lowercase letters, digits, '_' and '-', starting with a letter`,
				`commands[0].params[1].type: unknown type "date" (use one of: string, integer, number, boolean, json)`,
				"commands[0].params[2].enum: enum is only allowed for string and integer parameters",
				"commands[0].params[3].enum: enum must not be empty",
				"commands[0].params[4].enum[0]: enum value x is not an integer",
				"commands[0].params[5].default: default must be a list of JSON values, got string",
			},
		},
		{
			name: "output rules",
			m: Manifest{SchemaVersion: 1, Name: "pr", Commands: []Command{{Name: "a", Description: "d", Output: &Output{
				Table:  []Column{{Path: "a", JQ: ".a"}, {Header: "B"}},
				Detail: []DetailField{{Key: "K", Path: ""}, {Path: "x"}},
			}}}},
			want: []string{
				`commands[0].output.table[0]: missing required field "header"`,
				"commands[0].output.table[0]: set exactly one of path or jq",
				"commands[0].output.table[1]: set exactly one of path or jq",
				"commands[0].output.detail[0]: set exactly one of path or jq",
				`commands[0].output.detail[1]: missing required field "key"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSemantics(&tt.m, Options{}, nil)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("validateSemantics = %v", err)
			}
			if got := ve.Details(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("errors =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(tt.want, "\n  "))
			}
		})
	}
}

func TestValidateExtensionName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
	}{
		{"pr", ""},
		{"0day", ""},
		{"my-ext-2", ""},
		{strings.Repeat("a", 32), ""},
		{"", "must not be empty"},
		{strings.Repeat("a", 33), "longer than 32"},
		{"-pr", "is invalid"},
		{"Pr", "is invalid"},
		{"p_r", "is invalid"},
		{"extension", "reserved"},
		{"ext", "reserved"},
		{"help", "reserved"},
		{"completion", "reserved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExtensionName(tt.name)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateExtensionName(%q) = %v, want %q", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in   string
		want [3]int
		ok   bool
	}{
		{"2.9.0", [3]int{2, 9, 0}, true},
		{"v10.0.12", [3]int{10, 0, 12}, true},
		{"v2.9.0-4-gabc-dirty", [3]int{2, 9, 0}, true},
		{"2.9.0+build", [3]int{2, 9, 0}, true},
		{"dev", [3]int{}, false},
		{"", [3]int{}, false},
		{"2.9", [3]int{}, false},
		{"abc1234", [3]int{}, false},
	}
	for _, tt := range tests {
		got, ok := parseVersion(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("parseVersion(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
	if compareVersions([3]int{2, 9, 0}, [3]int{2, 10, 0}) != -1 ||
		compareVersions([3]int{3, 0, 0}, [3]int{2, 99, 99}) != 1 ||
		compareVersions([3]int{1, 2, 3}, [3]int{1, 2, 3}) != 0 {
		t.Error("compareVersions ordering is wrong")
	}
}

func TestTimeoutDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"", DefaultTimeout},
		{"30s", 30 * time.Second},
		{"1m30s", 90 * time.Second},
		{"bogus", DefaultTimeout},
		{"-5s", DefaultTimeout},
		{"1h", MaxTimeout},
	}
	for _, tt := range tests {
		c := Command{Timeout: tt.in}
		if got := c.TimeoutDuration(); got != tt.want {
			t.Errorf("TimeoutDuration(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestCommandAccessors(t *testing.T) {
	c := Command{Params: []Param{
		{Name: "a", Positional: true},
		{Name: "b"},
		{Name: "c", Positional: true},
		{Name: "d"},
	}}
	names := func(ps []*Param) (out []string) {
		for _, p := range ps {
			out = append(out, p.Name)
		}
		return out
	}
	if got := names(c.Positionals()); !reflect.DeepEqual(got, []string{"a", "c"}) {
		t.Errorf("Positionals = %v", got)
	}
	if got := names(c.FlagParams()); !reflect.DeepEqual(got, []string{"b", "d"}) {
		t.Errorf("FlagParams = %v", got)
	}
	if c.Param("d") == nil || c.Param("x") != nil {
		t.Error("Param lookup is wrong")
	}
	if !(&Command{}).MCPEnabled() || (&Command{MCP: boolPtr(false)}).MCPEnabled() || (&Command{Raw: true}).MCPEnabled() {
		t.Error("MCPEnabled is wrong")
	}
}

func TestErrorFormatting(t *testing.T) {
	tests := []struct {
		err  Error
		want string
	}{
		{Error{File: "f.yml", Line: 3, Path: "a.b", Message: "bad"}, "f.yml:3: a.b: bad"},
		{Error{File: "f.yml", Message: "bad"}, "f.yml: bad"},
		{Error{Line: 2, Message: "bad"}, "line 2: bad"},
		{Error{Path: "x", Message: "bad"}, "x: bad"},
		{Error{Message: "bad"}, "bad"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestPositionsLineFallsBackToAncestor(t *testing.T) {
	p := positions{"": 1, "/commands": 3, "/commands/0": 4}
	tests := map[string]int{
		"/commands/0/params/2/default": 4,
		"/commands":                    3,
		"/other":                       1,
		"":                             1,
	}
	for ptr, want := range tests {
		if got := p.line(ptr); got != want {
			t.Errorf("line(%q) = %d, want %d", ptr, got, want)
		}
	}
	if got := positions(nil).line("/a"); got != 0 {
		t.Errorf("nil positions line = %d", got)
	}
}

func TestDisplayPath(t *testing.T) {
	tests := map[string]string{
		"":                           "",
		"/name":                      "name",
		"/commands/0/params/12/type": "commands[0].params[12].type",
		"/commands/1/output/table/0": "commands[1].output.table[0]",
		"/issue_id":                  "issue_id",
	}
	for in, want := range tests {
		if got := displayPath(in); got != want {
			t.Errorf("displayPath(%q) = %q, want %q", in, got, want)
		}
	}
}
