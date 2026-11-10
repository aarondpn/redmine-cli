package spec

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func boolPtr(b bool) *bool { return &b }

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

// TestDuplicateToolNames covers the tool name check directly: the naming
// rules keep valid names from colliding, so Parse alone cannot reach it
// without also failing the command name pattern.
func TestDuplicateToolNames(t *testing.T) {
	m := &Manifest{SchemaVersion: 1, Name: "x", Commands: []Command{
		{Name: "a-b", Description: "d"},
		{Name: "a_b", Description: "d"},
		{Name: "c-d", Description: "d"},
		{Name: "c_d", Description: "d", MCP: boolPtr(false)},
	}}
	is := &issues{}
	validateSemantics(is, m, Options{})
	got := is.err()
	want := `commands[1].name: MCP tool name "x_a_b" is already used by command "a-b"; rename the command or set mcp: false`
	if got == nil || got.Error() != want {
		t.Fatalf("validateSemantics = %v, want %q", got, want)
	}

	src := header + "commands:\n  - {name: a-b, description: d}\n  - {name: a_b, description: d}\n"
	_, err := Parse([]byte(src), Options{})
	if err == nil || !strings.Contains(err.Error(), `redmine-extension.yml:5: commands[1].name: MCP tool name "pr_a_b" is already used by command "a-b"`) {
		t.Fatalf("Parse = %v", err)
	}
}
