package spec

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/spf13/cobra"
)

func TestYAMLScalarsBecomeJSONValues(t *testing.T) {
	tests := []struct {
		yaml string
		want any
	}{
		{"0x10", int64(16)},
		{"-7", int64(-7)},
		{"1.5e3", int64(1500)},
		{"2.5", 2.5},
		{"9223372036854775807", int64(9223372036854775807)},
		{"18446744073709551615", float64(18446744073709551615)},
		{"2024-01-01", "2024-01-01"},
		{"2024-01-01T10:00:00Z", "2024-01-01T10:00:00Z"},
		{"'007'", "007"},
		{"yes", "yes"},
		{"true", true},
		{"~", nil},
		{"!!binary aGk=", "hi"},
	}
	for _, tt := range tests {
		t.Run(tt.yaml, func(t *testing.T) {
			src := header + "commands:\n  - name: a\n    description: d\n    params:\n      - name: j\n        type: json\n        default: " + tt.yaml + "\n"
			m, err := Parse([]byte(src), Options{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := m.Commands[0].Params[0].Default; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("default = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestYAMLNonJSONNumbersRejected(t *testing.T) {
	for _, v := range []string{".inf", "-.inf", ".nan"} {
		src := header + "commands:\n  - name: a\n    description: d\n    params:\n      - {name: j, type: json, default: " + v + "}\n"
		_, err := Parse([]byte(src), Options{})
		if err == nil || !strings.Contains(err.Error(), "redmine-extension.yml:7: commands[0].params[0].default: "+v+" is not a valid JSON number") {
			t.Errorf("%s: %v", v, err)
		}
	}
}

func TestArrayFlagsOfEveryType(t *testing.T) {
	c := &Command{Name: "a", Description: "d", Params: []Param{
		{Name: "nums", Type: TypeNumber, Array: true, Default: []any{1.5}},
		{Name: "flags", Type: TypeBoolean, Array: true, Default: []any{true}},
		{Name: "docs", Type: TypeJSON, Array: true, Default: []any{map[string]any{"a": int64(1)}}},
		{Name: "names", Type: TypeString, Array: true, Default: []any{"x"}},
	}}
	cmd := &cobra.Command{Use: "a"}
	if err := AddFlags(cmd, c); err != nil {
		t.Fatal(err)
	}
	wantDefaults := map[string]string{"nums": "[1.500000]", "flags": "[true]", "docs": `[{"a":1}]`, "names": "[x]"}
	for name, want := range wantDefaults {
		if got := cmd.Flags().Lookup(name).DefValue; got != want {
			t.Errorf("--%s default = %q, want %q", name, got, want)
		}
	}

	got, err := runCommand(t, c, "", "--nums", "1,2.5", "--flags", "false", "--docs", "[]", "--docs", `"s"`, "--names", "a")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"nums": []any{1.0, 2.5}, "flags": []any{false},
		"docs": []any{[]any{}, "s"}, "names": []any{"a"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("input = %#v\nwant %#v", got, want)
	}

	if _, err := runCommand(t, c, "", "--docs", "{"); err == nil || !strings.Contains(err.Error(), "invalid JSON for --docs") {
		t.Errorf("bad json item: %v", err)
	}
}

func TestBuildInputFlagNotRegistered(t *testing.T) {
	c := &Command{Name: "a", Params: []Param{{Name: "x", Type: TypeString}}}
	cmd := &cobra.Command{Use: "a"}
	if _, err := BuildInput(cmd, c, nil); err == nil || !strings.Contains(err.Error(), "flag --x is not registered") {
		t.Fatalf("BuildInput = %v", err)
	}
}

func TestCoerceScalar(t *testing.T) {
	tests := []struct {
		t    ParamType
		v    any
		want any
		ok   bool
	}{
		{TypeInteger, 3, int64(3), true},
		{TypeInteger, float64(1 << 54), nil, false},
		{TypeInteger, "3", nil, false},
		{TypeNumber, 3, 3.0, true},
		{TypeNumber, int64(3), 3.0, true},
		{TypeNumber, "3", nil, false},
		{TypeBoolean, "true", false, false},
		{TypeJSON, nil, nil, true},
		{"date", "x", nil, false},
	}
	for _, tt := range tests {
		got, ok := coerceScalar(tt.t, tt.v)
		if ok != tt.ok || (ok && !reflect.DeepEqual(got, tt.want)) {
			t.Errorf("coerceScalar(%s, %#v) = %#v, %v; want %#v, %v", tt.t, tt.v, got, ok, tt.want, tt.ok)
		}
	}
}

func TestWiden(t *testing.T) {
	in := map[string]any{"a": []int64{1}, "b": []float64{1.5}, "c": []bool{true}, "d": []int{2}, "e": []string{"x"}, "f": "s"}
	want := map[string]any{"a": []any{int64(1)}, "b": []any{1.5}, "c": []any{true}, "d": []any{2}, "e": []any{"x"}, "f": "s"}
	if got := toAnyMap(in); !reflect.DeepEqual(got, want) {
		t.Errorf("toAnyMap = %#v", got)
	}
}

func TestCleanSchemaError(t *testing.T) {
	err := errors.New("validating https://redmine-cli.dev/schema/x.json: validating <anonymous schema>: type: 1 has type \"integer\", want \"string\"")
	if got := cleanSchemaError(err); got != `type: 1 has type "integer", want "string"` {
		t.Errorf("cleanSchemaError = %q", got)
	}
	if got := cleanSchemaError(errors.New("validating nothing")); got != "validating nothing" {
		t.Errorf("cleanSchemaError = %q", got)
	}
}

func TestSchemaCheckerFallsBackToLibraryMessage(t *testing.T) {
	// multipleOf has no hand-written message, so the library's is used.
	step := 2.0
	s := &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{"n": {Type: "integer", MultipleOf: &step}}}
	is := &issues{}
	if err := newSchemaChecker().check(is, "", map[string]any{"n": int64(3)}, s); err != nil {
		t.Fatal(err)
	}
	err := is.err()
	if err == nil || !strings.HasPrefix(err.Error(), "n: multipleOf: 3") {
		t.Fatalf("err = %v", err)
	}
}

func TestSchemaCheckerReportsBrokenSchema(t *testing.T) {
	s := &jsonschema.Schema{Ref: "#/definitions/missing"}
	err := newSchemaChecker().check(&issues{}, "", 1, s)
	var rerr *resolveError
	if !errors.As(err, &rerr) || !strings.HasPrefix(err.Error(), "resolve schema:") {
		t.Fatalf("check = %v", err)
	}
}
