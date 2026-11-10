package spec

import (
	"bytes"
	"encoding/json"
	"math"
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
				// --meta null counts as not given, as over MCP, so the
				// default applies.
				"issue_id": int64(7), "urls": []any{"u1", "u2"}, "limit": int64(50), "ratio": 1.25,
				"draft": false, "ids": []any{int64(3), int64(4), int64(5)}, "meta": map[string]any{"source": "cli"},
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

// TestIntegerFlagsAreBase10 keeps flags, positionals and MCP on one integer
// syntax: pflag's own integer flags would read 010 as 8 and accept 0x2.
func TestIntegerFlagsAreBase10(t *testing.T) {
	c := &Command{Name: "a", Description: "d", Params: []Param{
		{Name: "pos", Type: TypeInteger, Positional: true},
		{Name: "n", Type: TypeInteger, Default: int64(5)},
		{Name: "ids", Type: TypeInteger, Array: true, Default: []any{int64(1)}},
	}}
	got, err := runCommand(t, c, "", "010", "--n", "010", "--ids", "010,-3", "--ids", "+7")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"pos": int64(10), "n": int64(10), "ids": []any{int64(10), int64(-3), int64(7)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("input = %#v, want %#v", got, want)
	}

	got, err = runCommand(t, c, "")
	if err != nil || !reflect.DeepEqual(got, map[string]any{"n": int64(5), "ids": []any{int64(1)}}) {
		t.Errorf("defaults = %#v, %v", got, err)
	}

	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--n", "0x2"}, `invalid argument "0x2" for "--n" flag: must be an integer`},
		{[]string{"--n", "1_000"}, `invalid argument "1_000" for "--n" flag: must be an integer`},
		{[]string{"--ids", "1,0o7"}, `invalid argument "1,0o7" for "--ids" flag: must be an integer`},
		{[]string{"--n", "9223372036854775808"}, "must be an integer between -9223372036854775808 and 9223372036854775807"},
		{[]string{"0x2"}, `invalid value "0x2" for <pos>: must be an integer`},
	} {
		if _, err := runCommand(t, c, "", tt.args...); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("args %v: error = %v, want containing %q", tt.args, err, tt.want)
		}
	}

	cmd := &cobra.Command{Use: "a"}
	if err := AddFlags(cmd, c); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"n": "5", "ids": "[1]"} {
		if f := cmd.Flags().Lookup(name); f.DefValue != want {
			t.Errorf("--%s default = %q, want %q", name, f.DefValue, want)
		}
	}
	if typ := cmd.Flags().Lookup("n").Value.Type(); typ != "int64" {
		t.Errorf("--n type = %q", typ)
	}
}

// TestJSONNullIsAbsent: a json param given as null counts as not given on
// the CLI, as it does over MCP.
func TestJSONNullIsAbsent(t *testing.T) {
	c := &Command{Name: "a", Description: "d", Params: []Param{
		{Name: "req", Type: TypeJSON, Required: true},
		{Name: "opt", Type: TypeJSON, Default: map[string]any{"d": true}},
		{Name: "pos", Type: TypeJSON, Positional: true},
	}}
	if _, err := runCommand(t, c, "", "--req", "null"); err == nil || err.Error() != "missing required flag --req" {
		t.Fatalf("required null: %v", err)
	}
	cli, err := runCommand(t, c, "", "null", "--req", "1", "--opt", "null")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"req": json.Number("1"), "opt": map[string]any{"d": true}}
	if !reflect.DeepEqual(cli, want) {
		t.Errorf("CLI input = %#v, want %#v", cli, want)
	}

	args, err := DecodeArguments([]byte(`{"req":1,"opt":null,"pos":null}`))
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := PrepareInput(c, NormalizeArguments(c, args))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mcp, cli) {
		t.Errorf("MCP input = %#v, CLI input = %#v", mcp, cli)
	}
	if _, err := PrepareInput(c, map[string]any{"req": nil}); err == nil || !strings.Contains(err.Error(), `missing required field "req"`) {
		t.Errorf("MCP required null: %v", err)
	}
}

func TestBooleanArrayFlagsRejected(t *testing.T) {
	c := &Command{Params: []Param{{Name: "b", Type: TypeBoolean, Array: true}}}
	if err := AddFlags(&cobra.Command{Use: "x"}, c); err == nil || !strings.Contains(err.Error(), "boolean parameters cannot be arrays") {
		t.Fatalf("AddFlags = %v", err)
	}
}

func TestNumberFlagsUseJSONGrammar(t *testing.T) {
	c := &Command{Name: "a", Description: "d", Params: []Param{
		{Name: "pos", Type: TypeNumber, Positional: true},
		{Name: "x", Type: TypeNumber, Default: 1.5},
		{Name: "xs", Type: TypeNumber, Array: true},
	}}
	got, err := runCommand(t, c, "", "0.25", "--x", "2e3", "--xs", "1,2.5", "--xs", "-3")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"pos": 0.25, "x": 2000.0, "xs": []any{1.0, 2.5, -3.0}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("input = %#v, want %#v", got, want)
	}

	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--x", "NaN"}, `invalid argument "NaN" for "--x" flag: must be a number`},
		{[]string{"--x", "Inf"}, `invalid argument "Inf" for "--x" flag: must be a number`},
		{[]string{"--x", "0x1p-2"}, `invalid argument "0x1p-2" for "--x" flag: must be a number`},
		{[]string{"--x", "1_000"}, `invalid argument "1_000" for "--x" flag: must be a number`},
		{[]string{"--xs", "1,nan"}, `invalid argument "1,nan" for "--xs" flag: must be a number`},
		{[]string{"--x", "1e400"}, "must be a finite number"},
		{[]string{"+Inf"}, `invalid value "+Inf" for <pos>: must be a number`},
	} {
		if _, err := runCommand(t, c, "", tt.args...); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("args %v: error = %v, want containing %q", tt.args, err, tt.want)
		}
	}

	cmd := &cobra.Command{Use: "a"}
	if err := AddFlags(cmd, c); err != nil {
		t.Fatal(err)
	}
	if f := cmd.Flags().Lookup("x"); f.DefValue != "1.5" || f.Value.Type() != "float64" {
		t.Errorf("--x default/type = %q/%q, want 1.5/float64", f.DefValue, f.Value.Type())
	}
}

func TestCoerceNumberRejectsNonFinite(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, ok := coerceScalar(TypeNumber, v); ok {
			t.Errorf("coerceScalar(number, %v) accepted a non-finite value", v)
		}
	}
}
