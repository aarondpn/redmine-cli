package runner

import (
	"slices"
	"strings"
	"testing"
)

func TestEnvVars(t *testing.T) {
	tests := []struct {
		name string
		env  Env
		want []string
	}{
		{
			name: "minimal cli",
			env:  Env{CLIBin: "/bin/redmine", Extension: "pr"},
			want: []string{
				"REDMINE_CLI_BIN=/bin/redmine",
				"REDMINE_EXT_NAME=pr",
				"REDMINE_EXT_SURFACE=cli",
				"REDMINE_NO_UPDATE_CHECK=1",
			},
		},
		{
			name: "cli with every override",
			env: Env{
				CLIBin: "/bin/redmine", Extension: "pr", Command: "list", Surface: SurfaceCLI,
				Tool:    "ignored_on_cli",
				Profile: "work", ConfigPath: "/cfg.yaml", Server: "https://r.example.com",
				Headers:  []string{"X-A: 1", "X-B: two words"},
				ReadOnly: true, NoColor: true,
			},
			want: []string{
				"REDMINE_CLI_BIN=/bin/redmine",
				"REDMINE_EXT_NAME=pr",
				"REDMINE_EXT_COMMAND=list",
				"REDMINE_EXT_SURFACE=cli",
				"REDMINE_PROFILE=work",
				"REDMINE_CONFIG=/cfg.yaml",
				"REDMINE_SERVER=https://r.example.com",
				"REDMINE_HEADERS=X-A: 1\nX-B: two words",
				"REDMINE_READ_ONLY=1",
				"REDMINE_NO_COLOR=1",
				"REDMINE_NO_UPDATE_CHECK=1",
			},
		},
		{
			name: "mcp surface",
			env:  Env{CLIBin: "/bin/redmine", Extension: "pr", Command: "list", Surface: SurfaceMCP, Tool: "pr_list", ReadOnly: true},
			want: []string{
				"REDMINE_CLI_BIN=/bin/redmine",
				"REDMINE_EXT_NAME=pr",
				"REDMINE_EXT_COMMAND=list",
				"REDMINE_EXT_SURFACE=mcp",
				"REDMINE_MCP=1",
				"REDMINE_MCP_TOOL=pr_list",
				"REDMINE_READ_ONLY=1",
				"REDMINE_NO_UPDATE_CHECK=1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.env.vars(); !slices.Equal(got, tt.want) {
				t.Fatalf("vars() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestEnvEnviron(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"REDMINE_API_KEY=secret",
		"REDMINE_PROFILE=outer",
		"REDMINE_EXT_COMMAND=outer-cmd",
		"REDMINE_MCP=1",
		"REDMINE_MCP_TOOL=outer_tool",
		"REDMINE_EXT_SURFACE=mcp",
		"REDMINE_HEADERS=X-Outer: 1",
		"REDMINE_NO_UPDATE_CHECK=0",
	}

	tests := []struct {
		name      string
		base      []string
		env       Env
		want      map[string]string
		wantUnset []string
	}{
		{
			name: "contract replaces inherited values and strips stale ones",
			base: base,
			env:  Env{CLIBin: "/bin/redmine", Extension: "pr", Profile: "inner"},
			want: map[string]string{
				"PATH":                    "/usr/bin",
				"REDMINE_API_KEY":         "secret",
				"REDMINE_PROFILE":         "inner",
				"REDMINE_EXT_SURFACE":     "cli",
				"REDMINE_HEADERS":         "X-Outer: 1",
				"REDMINE_NO_UPDATE_CHECK": "1",
			},
			wantUnset: []string{"REDMINE_EXT_COMMAND", "REDMINE_MCP", "REDMINE_MCP_TOOL"},
		},
		{
			name: "headers override the inherited list",
			base: base,
			env:  Env{CLIBin: "/b", Extension: "pr", Headers: []string{"X-Outer: 1", "X-Flag: 2"}},
			want: map[string]string{"REDMINE_HEADERS": "X-Outer: 1\nX-Flag: 2", "REDMINE_PROFILE": "outer"},
		},
		{
			name: "explicitly disabled read-only clears an inherited one",
			base: []string{"REDMINE_READ_ONLY=true"},
			env:  Env{CLIBin: "/b", Extension: "pr"},
			want: map[string]string{"REDMINE_READ_ONLY": "0"},
		},
		{
			name: "inherited falsy read-only is kept",
			base: []string{"REDMINE_READ_ONLY=0"},
			env:  Env{CLIBin: "/b", Extension: "pr"},
			want: map[string]string{"REDMINE_READ_ONLY": "0"},
		},
		{
			name:      "no read-only stays unset",
			env:       Env{CLIBin: "/b", Extension: "pr"},
			wantUnset: []string{"REDMINE_READ_ONLY", "REDMINE_PROFILE", "REDMINE_CONFIG", "REDMINE_SERVER", "REDMINE_HEADERS", "REDMINE_NO_COLOR"},
		},
		{
			// --read-only=false with a read_only profile: the nested call
			// loads the same profile and must not become read-only again.
			name: "explicitly disabled read-only without an inherited one",
			env:  Env{CLIBin: "/b", Extension: "pr", ReadOnlyDisabled: true},
			want: map[string]string{"REDMINE_READ_ONLY": "0"},
		},
		{
			name: "explicitly disabled read-only replaces an inherited one",
			base: []string{"REDMINE_READ_ONLY=1"},
			env:  Env{CLIBin: "/b", Extension: "pr", ReadOnlyDisabled: true},
			want: map[string]string{"REDMINE_READ_ONLY": "0"},
		},
		{
			name: "forced read-only wins over an explicit disable",
			env:  Env{CLIBin: "/b", Extension: "pr", ReadOnly: true, ReadOnlyDisabled: true},
			want: map[string]string{"REDMINE_READ_ONLY": "1"},
		},
		{
			name: "forced read-only",
			base: []string{"REDMINE_READ_ONLY=0"},
			env:  Env{CLIBin: "/b", Extension: "pr", ReadOnly: true},
			want: map[string]string{"REDMINE_READ_ONLY": "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.env.Environ(tt.base)
			seen := map[string]int{}
			for _, kv := range got {
				k, _, _ := strings.Cut(kv, "=")
				seen[k]++
				if seen[k] > 1 {
					t.Fatalf("%s appears twice in %q", k, got)
				}
			}
			for k, v := range tt.want {
				if gotV, ok := lookup(got, k); !ok || gotV != v {
					t.Errorf("%s = %q (set %v), want %q", k, gotV, ok, v)
				}
			}
			for _, k := range tt.wantUnset {
				if v, ok := lookup(got, k); ok {
					t.Errorf("%s = %q, want unset", k, v)
				}
			}
		})
	}
}
