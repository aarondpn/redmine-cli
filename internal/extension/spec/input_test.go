package spec

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func loadExample(t *testing.T) *Manifest {
	t.Helper()
	m, err := ParseFile(filepath.Join("testdata", FileName), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return m
}

func TestToolName(t *testing.T) {
	tests := []struct{ ext, cmd, want string }{
		{"pr", "list", "pr_list"},
		{"my-ext", "link-pr", "my_ext_link_pr"},
		{"a", "b_c", "a_b_c"},
	}
	for _, tt := range tests {
		if got := ToolName(tt.ext, tt.cmd); got != tt.want {
			t.Errorf("ToolName(%q, %q) = %q, want %q", tt.ext, tt.cmd, got, tt.want)
		}
	}
}

func TestFlagName(t *testing.T) {
	for in, want := range map[string]string{"issue_id": "issue-id", "state": "state", "a_b-c": "a-b-c"} {
		if got := FlagName(in); got != want {
			t.Errorf("FlagName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInputSchema(t *testing.T) {
	m := loadExample(t)
	tests := []struct {
		cmd  string
		want string
	}{
		{
			cmd: "list",
			want: `{"type":"object","properties":{` +
				`"issue_id":{"type":"integer","description":"Redmine issue ID"},` +
				`"state":{"type":"string","default":"open","enum":["open","merged","closed"]},` +
				`"label":{"type":"array","items":{"type":"string"}},` +
				`"extra":{"description":"Any JSON value."}},` +
				`"required":["issue_id"],"additionalProperties":false}`,
		},
		{
			cmd: "link-pr",
			want: `{"type":"object","properties":{` +
				`"issue_id":{"type":"integer"},` +
				`"urls":{"type":"array","items":{"type":"string"},"minItems":1},` +
				`"limit":{"type":"integer","default":10,"enum":[10,20,50]},` +
				`"ratio":{"type":"number","default":0.5},` +
				`"draft":{"type":"boolean","default":true},` +
				`"ids":{"type":"array","items":{"type":"integer"},"default":[1,2]},` +
				`"meta":{"description":"Any JSON value.","default":{"source":"cli"}}},` +
				`"required":["issue_id","urls"],"additionalProperties":false}`,
		},
		{
			cmd:  "shell",
			want: `{"type":"object","properties":{},"additionalProperties":false}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			s := InputSchema(m.Command(tt.cmd))
			got, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("InputSchema =\n%s\nwant\n%s", got, tt.want)
			}
			if _, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true}); err != nil {
				t.Errorf("Resolve: %v", err)
			}
		})
	}
}

func TestValidateInput(t *testing.T) {
	m := loadExample(t)
	list := m.Command("list")
	tests := []struct {
		name  string
		input map[string]any
		want  []string
	}{
		{name: "minimal", input: map[string]any{"issue_id": 1}},
		{name: "float integer from JSON", input: map[string]any{"issue_id": float64(12)}},
		{name: "all", input: map[string]any{"issue_id": int64(1), "state": "merged", "label": []any{"a"}, "extra": map[string]any{"x": nil}}},
		{name: "typed slice", input: map[string]any{"issue_id": 1, "label": []string{"a", "b"}}},
		{name: "missing required", input: map[string]any{}, want: []string{`missing required field "issue_id"`}},
		{name: "wrong type", input: map[string]any{"issue_id": "12"}, want: []string{"issue_id: must be an integer, got string"}},
		{name: "fractional integer", input: map[string]any{"issue_id": 1.5}, want: []string{"issue_id: must be an integer, got number"}},
		{name: "enum", input: map[string]any{"issue_id": 1, "state": "draft"}, want: []string{"state: must be one of: open, merged, closed (got draft)"}},
		{name: "array item", input: map[string]any{"issue_id": 1, "label": []any{"a", 2}}, want: []string{"label[1]: must be a string, got integer"}},
		{name: "unknown", input: map[string]any{"issue_id": 1, "bogus": true}, want: []string{`unknown field "bogus"`}},
		{name: "null", input: map[string]any{"issue_id": nil}, want: []string{"issue_id: must be an integer, got null"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInput(list, tt.input)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("ValidateInput: %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("ValidateInput = %v", err)
			}
			if got := ve.Details(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("errors = %q, want %q", got, tt.want)
			}
		})
	}

	link := m.Command("link-pr")
	if err := ValidateInput(link, map[string]any{"issue_id": 1, "urls": []any{}}); err == nil || !strings.Contains(err.Error(), "urls: must not be empty") {
		t.Errorf("empty required array: %v", err)
	}
}

func TestPrepareInput(t *testing.T) {
	m := loadExample(t)
	tests := []struct {
		name    string
		cmd     string
		args    map[string]any
		want    map[string]any
		wantErr string
	}{
		{
			name: "defaults applied, nulls dropped, integers normalized",
			cmd:  "list",
			args: map[string]any{"issue_id": float64(42), "label": nil, "extra": nil},
			want: map[string]any{"issue_id": int64(42), "state": "open"},
		},
		{
			name: "explicit values win",
			cmd:  "link-pr",
			args: map[string]any{"issue_id": float64(1), "urls": []any{"u"}, "limit": float64(20), "ids": []any{float64(7)}, "ratio": float64(2), "meta": []any{1.5}},
			want: map[string]any{
				"issue_id": int64(1), "urls": []any{"u"}, "limit": int64(20), "ids": []any{int64(7)},
				"ratio": float64(2), "draft": true, "meta": []any{1.5},
			},
		},
		{
			name:    "invalid",
			cmd:     "link-pr",
			args:    map[string]any{"issue_id": float64(1), "urls": []any{"u"}, "limit": float64(15)},
			wantErr: "limit: must be one of: 10, 20, 50 (got 15)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PrepareInput(m.Command(tt.cmd), tt.args)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("PrepareInput error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrepareInput = %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestApplyDefaultsCopies(t *testing.T) {
	m := loadExample(t)
	link := m.Command("link-pr")
	in := map[string]any{"issue_id": int64(1)}
	out := ApplyDefaults(link, in)
	if len(in) != 1 {
		t.Error("ApplyDefaults modified its input")
	}
	out["ids"].([]any)[0] = int64(99)
	out["meta"].(map[string]any)["source"] = "changed"
	if link.Param("ids").Default.([]any)[0] != int64(1) || link.Param("meta").Default.(map[string]any)["source"] != "cli" {
		t.Error("ApplyDefaults shares default values with the manifest")
	}
	if got := ApplyDefaults(m.Command("unlink"), nil); len(got) != 0 {
		t.Errorf("ApplyDefaults(nil) = %v", got)
	}
}

func TestCheckValue(t *testing.T) {
	p := &Param{Name: "x", Type: TypeInteger, Enum: []any{int64(1), int64(2)}}
	tests := []struct {
		v    any
		arr  bool
		want string
	}{
		{int64(1), false, ""},
		{float64(2), false, ""},
		{json.Number("2"), false, ""},
		{int64(3), false, "must be one of: 1, 2 (got 3)"},
		{"1", false, "must be an integer, got string"},
		{[]any{int64(1)}, true, ""},
		{int64(1), true, "must be a list of integers, got integer"},
		{[]any{int64(1), true}, true, "item 2 must be an integer, got boolean"},
	}
	for _, tt := range tests {
		p.Array = tt.arr
		err := checkValue(p, tt.v)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("checkValue(%#v, array=%v) = %q, want %q", tt.v, tt.arr, got, tt.want)
		}
	}
}
