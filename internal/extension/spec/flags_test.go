package spec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runCommand configures a cobra command for c, executes it with args and
// returns the input object BuildInput produced.
func runCommand(t *testing.T, c *Command, stdin string, args ...string) (map[string]any, error) {
	t.Helper()
	var got map[string]any
	cmd := &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := BuildInput(cmd, c, args)
			got = in
			return err
		},
	}
	if err := Configure(cmd, c); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return got, err
}

func TestBuildInput(t *testing.T) {
	m := loadExample(t)
	dir := t.TempDir()
	jsonFile := filepath.Join(dir, "extra.json")
	if err := os.WriteFile(jsonFile, []byte(`{"n": 12345678901234567890, "list": [1, 2.5, 1.0, 1e2]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		cmd     string
		stdin   string
		args    []string
		want    map[string]any
		wantErr string
	}{
		{
			name: "positional and defaults",
			cmd:  "list",
			args: []string{"42"},
			want: map[string]any{"issue_id": int64(42), "state": "open"},
		},
		{
			name: "flags",
			cmd:  "list",
			args: []string{"42", "--state", "merged", "--label", "a,b", "--label", "c", "--extra", `{"x":[1,true]}`},
			want: map[string]any{
				"issue_id": int64(42), "state": "merged", "label": []any{"a,b", "c"},
				"extra": map[string]any{"x": []any{json.Number("1"), true}},
			},
		},
		{
			name: "json from file",
			cmd:  "list",
			args: []string{"1", "--extra", "@" + jsonFile},
			want: map[string]any{
				"issue_id": int64(1), "state": "open",
				// Numbers in a json value keep their exact text, like
				// over MCP: no int64 overflow, no float rounding.
				"extra": map[string]any{"n": json.Number("12345678901234567890"), "list": []any{json.Number("1"), json.Number("2.5"), json.Number("1.0"), json.Number("1e2")}},
			},
		},
		{
			name:  "json from stdin",
			cmd:   "list",
			stdin: `"text"`,
			args:  []string{"1", "--extra", "@-"},
			want:  map[string]any{"issue_id": int64(1), "state": "open", "extra": "text"},
		},
		{
			name: "array positional and typed flags",
			cmd:  "link-pr",
			args: []string{"7", "u1", "u2", "--limit", "50", "--ratio", "1.25", "--draft=false", "--ids", "3,4", "--ids", "5", "--meta", "null"},
			want: map[string]any{
				"issue_id": int64(7), "urls": []any{"u1", "u2"}, "limit": int64(50), "ratio": 1.25,
				"draft": false, "ids": []any{int64(3), int64(4), int64(5)}, "meta": nil,
			},
		},
		{
			name: "defaults of every type",
			cmd:  "link-pr",
			args: []string{"7", "u1"},
			want: map[string]any{
				"issue_id": int64(7), "urls": []any{"u1"}, "limit": int64(10), "ratio": 0.5,
				"draft": true, "ids": []any{int64(1), int64(2)}, "meta": map[string]any{"source": "cli"},
			},
		},
		{name: "missing positional", cmd: "list", args: nil, wantErr: "missing required argument <issue-id>"},
		{name: "missing array positional", cmd: "link-pr", args: []string{"7"}, wantErr: "missing required argument <urls>"},
		{name: "too many args", cmd: "list", args: []string{"1", "2"}, wantErr: `accepts at most 1 argument(s), received 2`},
		{name: "bad integer positional", cmd: "list", args: []string{"abc"}, wantErr: `invalid value "abc" for <issue-id>: must be an integer`},
		{name: "enum flag", cmd: "list", args: []string{"1", "--state", "draft"}, wantErr: "invalid value for --state: must be one of: open, merged, closed (got draft)"},
		{name: "integer enum flag", cmd: "link-pr", args: []string{"1", "u", "--limit", "15"}, wantErr: "invalid value for --limit: must be one of: 10, 20, 50 (got 15)"},
		{name: "invalid json", cmd: "list", args: []string{"1", "--extra", "{nope"}, wantErr: "invalid JSON for --extra (value)"},
		{name: "trailing json", cmd: "list", args: []string{"1", "--extra", "1 2"}, wantErr: "unexpected data after the value"},
		{name: "missing file", cmd: "list", args: []string{"1", "--extra", "@" + filepath.Join(dir, "nope.json")}, wantErr: "--extra: open"},
		{name: "bare @", cmd: "list", args: []string{"1", "--extra", "@"}, wantErr: "@ must be followed by a file path or -"},
		{name: "pflag type error", cmd: "link-pr", args: []string{"1", "u", "--ratio", "x"}, wantErr: `invalid argument "x" for "--ratio"`},
		{name: "no args allowed", cmd: "unlink", args: []string{"1", "2"}, wantErr: "accepts at most 1 argument(s), received 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCommand(t, m.Command(tt.cmd), tt.stdin, tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("input = %#v\nwant    %#v", got, tt.want)
			}
		})
	}
}

func TestBuildInputRequiredFlag(t *testing.T) {
	c := &Command{Name: "a", Description: "d", Params: []Param{{Name: "project_id", Type: TypeString, Required: true}}}
	if _, err := runCommand(t, c, ""); err == nil || err.Error() != "missing required flag --project-id" {
		t.Fatalf("error = %v", err)
	}
	got, err := runCommand(t, c, "", "--project-id", "web")
	if err != nil || !reflect.DeepEqual(got, map[string]any{"project_id": "web"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestBuildInputOneStdinValue(t *testing.T) {
	c := &Command{Name: "a", Description: "d", Params: []Param{{Name: "data", Type: TypeJSON, Array: true}}}
	if _, err := runCommand(t, c, "1", "--data", "@-", "--data", "@-"); err == nil || !strings.Contains(err.Error(), "only one value can be read from stdin") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildInputPositionalTypes(t *testing.T) {
	c := &Command{Name: "a", Description: "d", Params: []Param{
		{Name: "n", Type: TypeNumber, Positional: true, Required: true},
		{Name: "b", Type: TypeBoolean, Positional: true, Required: true},
		{Name: "j", Type: TypeJSON, Positional: true},
		{Name: "rest", Type: TypeInteger, Positional: true, Array: true},
	}}
	got, err := runCommand(t, c, "", "1.5", "true", `{"a":1}`, "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"n": 1.5, "b": true, "j": map[string]any{"a": json.Number("1")}, "rest": []any{int64(1), int64(2)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("input = %#v", got)
	}
	for _, args := range [][]string{{"x", "true"}, {"1", "maybe"}, {"1", "true", "{"}, {"1", "true", "null", "x"}} {
		if _, err := runCommand(t, c, "", args...); err == nil {
			t.Errorf("args %v: expected error", args)
		}
	}
}

func TestUseLineAndArgs(t *testing.T) {
	m := loadExample(t)
	tests := map[string]string{
		"list":    "list <issue-id>",
		"link-pr": "link-pr <issue-id> <urls...>",
		"shell":   "shell",
	}
	for name, want := range tests {
		if got := UseLine(m.Command(name)); got != want {
			t.Errorf("UseLine(%s) = %q, want %q", name, got, want)
		}
	}
	opt := &Command{Name: "x", Params: []Param{{Name: "a", Positional: true}, {Name: "b", Positional: true, Array: true}}}
	if got := UseLine(opt); got != "x [a] [b...]" {
		t.Errorf("UseLine = %q", got)
	}
	args := PositionalArgs(opt)
	for _, n := range []int{0, 1, 5} {
		if err := args(&cobra.Command{Use: "x"}, make([]string, n)); err != nil {
			t.Errorf("%d args: %v", n, err)
		}
	}
	none := PositionalArgs(&Command{Name: "y"})
	if err := none(&cobra.Command{Use: "y"}, []string{"a"}); err == nil || !strings.Contains(err.Error(), "accepts no arguments") {
		t.Errorf("no positionals: %v", err)
	}
}

func TestAddFlagsHelpAndDefaults(t *testing.T) {
	m := loadExample(t)
	cmd := &cobra.Command{Use: "x"}
	if err := Configure(cmd, m.Command("link-pr")); err != nil {
		t.Fatal(err)
	}
	if cmd.Short != "Link a pull request to an issue" || cmd.Args == nil {
		t.Errorf("Configure did not set Short/Args: %+v", cmd)
	}
	wantDefaults := map[string]string{
		"limit": "10", "ratio": "0.5", "draft": "true", "ids": "[1,2]", "meta": `{"source":"cli"}`,
	}
	for name, want := range wantDefaults {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("flag --%s missing", name)
		}
		if f.DefValue != want {
			t.Errorf("--%s default = %q, want %q", name, f.DefValue, want)
		}
	}
	if cmd.Flags().Lookup("issue-id") != nil || cmd.Flags().Lookup("urls") != nil {
		t.Error("positionals must not become flags")
	}

	list := &cobra.Command{Use: "x"}
	if err := AddFlags(list, m.Command("list")); err != nil {
		t.Fatal(err)
	}
	usage := map[string]string{
		"state": "(one of: open, merged, closed)",
		"label": "(repeatable)",
		"extra": "(JSON value or @file)",
	}
	for name, want := range usage {
		if got := list.Flags().Lookup(name).Usage; got != want {
			t.Errorf("--%s usage = %q, want %q", name, got, want)
		}
	}
	req := &Command{Params: []Param{{Name: "a", Type: TypeString, Description: "Thing", Required: true, Array: true}}}
	if got := flagUsage(&req.Params[0]); got != "Thing (repeatable; required)" {
		t.Errorf("flagUsage = %q", got)
	}
}

func TestAddFlagsRejectsClashes(t *testing.T) {
	reserved := &Command{Params: []Param{{Name: "output", Type: TypeString}}}
	if err := AddFlags(&cobra.Command{Use: "x"}, reserved); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Errorf("reserved flag: %v", err)
	}

	existing := &cobra.Command{Use: "x"}
	existing.Flags().String("state", "", "")
	if err := AddFlags(existing, &Command{Params: []Param{{Name: "state", Type: TypeString}}}); err == nil || !strings.Contains(err.Error(), "already defined") {
		t.Errorf("existing flag: %v", err)
	}

	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("custom", "", "")
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)
	if err := AddFlags(child, &Command{Params: []Param{{Name: "custom", Type: TypeString}}}); err == nil || !strings.Contains(err.Error(), "already defined") {
		t.Errorf("inherited flag: %v", err)
	}

	if err := AddFlags(&cobra.Command{Use: "x"}, &Command{Params: []Param{{Name: "a", Type: "date"}}}); err == nil {
		t.Error("unknown type accepted")
	}
}

func TestCompletion(t *testing.T) {
	m := loadExample(t)
	c := &Command{Name: "a", Description: "d", Params: []Param{
		{Name: "kind", Type: TypeString, Positional: true, Required: true, Enum: []any{"bug", "feature"}},
		{Name: "flag", Type: TypeBoolean, Positional: true},
		{Name: "data", Type: TypeJSON, Positional: true},
		{Name: "rest", Type: TypeInteger, Positional: true, Array: true, Enum: []any{int64(1), int64(2)}},
	}}
	cmd := &cobra.Command{Use: "a"}
	if err := AddFlags(cmd, c); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		args []string
		want []string
		dir  cobra.ShellCompDirective
	}{
		{nil, []string{"bug", "feature"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"bug"}, []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"bug", "true"}, nil, cobra.ShellCompDirectiveDefault},
		{[]string{"bug", "true", "{}"}, []string{"1", "2"}, cobra.ShellCompDirectiveNoFileComp},
		{[]string{"bug", "true", "{}", "1", "2"}, []string{"1", "2"}, cobra.ShellCompDirectiveNoFileComp},
	}
	for _, tt := range tests {
		got, dir := cmd.ValidArgsFunction(cmd, tt.args, "")
		if !reflect.DeepEqual(got, tt.want) || dir != tt.dir {
			t.Errorf("complete(%v) = %v, %v; want %v, %v", tt.args, got, dir, tt.want, tt.dir)
		}
	}

	plain := &cobra.Command{Use: "list"}
	if err := AddFlags(plain, m.Command("list")); err != nil {
		t.Fatal(err)
	}
	if got, dir := plain.ValidArgsFunction(plain, []string{"1"}, ""); got != nil || dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("beyond last positional = %v, %v", got, dir)
	}
	fn, ok := plain.GetFlagCompletionFunc("state")
	if !ok {
		t.Fatal("no completion for --state")
	}
	if got, _ := fn(plain, nil, ""); !reflect.DeepEqual(got, []string{"open", "merged", "closed"}) {
		t.Errorf("--state completion = %v", got)
	}
	if _, ok := plain.GetFlagCompletionFunc("label"); ok {
		t.Error("--label should have no fixed completion")
	}
}
