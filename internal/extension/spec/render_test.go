package spec

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func decodeJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRender(t *testing.T) {
	m := loadExample(t)
	spec := m.Command("list").Output
	tests := []struct {
		name   string
		out    *Output
		result string
		want   *Rendered
	}{
		{
			name: "table",
			out:  spec,
			result: `[{"id": 12345678901234567, "title": "Fix", "author": {"login": "ann"}},
			          {"id": 2, "title": null}]`,
			want: &Rendered{
				View:    ViewTable,
				Headers: []string{"ID", "Title", "Author"},
				Rows:    [][]string{{"12345678901234567", "Fix", "ann"}, {"2", "", ""}},
			},
		},
		{
			name:   "empty array is still a table",
			out:    spec,
			result: `[]`,
			want:   &Rendered{View: ViewTable, Headers: []string{"ID", "Title", "Author"}, Rows: [][]string{}},
		},
		{
			name:   "detail",
			out:    spec,
			result: `{"state": "open"}`,
			want:   &Rendered{View: ViewDetail, Pairs: []output.KeyValue{{Key: "State", Value: "open"}}},
		},
		{name: "scalar", out: spec, result: `"done"`, want: &Rendered{View: ViewJSON}},
		{name: "no spec", out: nil, result: `[1]`, want: &Rendered{View: ViewJSON}},
		{name: "empty spec", out: &Output{}, result: `[1]`, want: &Rendered{View: ViewJSON}},
		{name: "array without table spec", out: &Output{Detail: spec.Detail}, result: `[1]`, want: &Rendered{View: ViewJSON}},
		{name: "object without detail spec", out: &Output{Table: spec.Table}, result: `{}`, want: &Rendered{View: ViewJSON}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Decode with json.Number, as callers reading extension stdout do.
			var v any
			dec := json.NewDecoder(strings.NewReader(tt.result))
			dec.UseNumber()
			if err := dec.Decode(&v); err != nil {
				t.Fatal(err)
			}
			got, err := Render(context.Background(), tt.out, v)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Render = %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestRenderSelectors(t *testing.T) {
	item := decodeJSON(t, `{
		"id": 7, "ratio": 0.25, "ok": true, "none": null,
		"labels": ["bug", "ui"],
		"assignees": [{"login": "a"}, {"login": "b"}],
		"meta": {"k": "v"},
		"nested": {"list": [{"name": "first"}]}
	}`)
	tests := []struct {
		name string
		col  Column
		want string
	}{
		{"integer", Column{Path: "id"}, "7"},
		{"float", Column{Path: "ratio"}, "0.25"},
		{"bool", Column{Path: "ok"}, "true"},
		{"null", Column{Path: "none"}, ""},
		{"missing", Column{Path: "nope.deeper"}, ""},
		{"scalar list", Column{Path: "labels"}, "bug, ui"},
		{"object list", Column{Path: "assignees"}, `[{"login":"a"},{"login":"b"}]`},
		{"object", Column{Path: "meta"}, `{"k":"v"}`},
		{"index", Column{Path: "nested.list.0.name"}, "first"},
		{"index out of range", Column{Path: "labels.5"}, ""},
		{"non-numeric index", Column{Path: "labels.x"}, ""},
		{"through scalar", Column{Path: "id.x"}, ""},
		{"jq scalar", Column{JQ: ".id + 1"}, "8"},
		{"jq multiple results", Column{JQ: ".assignees[].login"}, "a, b"},
		{"jq join", Column{JQ: `.labels | join("/")`}, "bug/ui"},
		{"jq null skipped", Column{JQ: ".none"}, ""},
		{"jq object", Column{JQ: "{a: .id}"}, `{"a":7}`},
		{"jq empty", Column{JQ: "empty"}, ""},
		{"jq halt", Column{JQ: "halt"}, ""},
		{"jq has no env", Column{JQ: "env | length"}, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.col.Header = "H"
			got, err := Render(context.Background(), &Output{Table: []Column{tt.col}}, []any{item})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got.Rows[0][0] != tt.want {
				t.Errorf("cell = %q, want %q", got.Rows[0][0], tt.want)
			}
		})
	}
}

func TestRenderErrors(t *testing.T) {
	tests := []struct {
		name   string
		out    *Output
		result any
		want   string
	}{
		{"jq runtime error", &Output{Table: []Column{{Header: "X", JQ: ".a.b"}}}, []any{map[string]any{"a": "s"}}, `column "X": jq:`},
		{"jq compile error", &Output{Table: []Column{{Header: "X", JQ: ".a |"}}}, []any{1}, `column "X": invalid jq expression`},
		{"both selectors", &Output{Detail: []DetailField{{Key: "K", Path: "a", JQ: ".a"}}}, map[string]any{}, `field "K": set exactly one of path or jq`},
		{"empty path", &Output{Detail: []DetailField{{Key: "K"}}}, map[string]any{}, `field "K": path must not be empty`},
		{"detail jq error", &Output{Detail: []DetailField{{Key: "K", JQ: `error("boom")`}}}, map[string]any{}, `field "K": jq: error: boom`},
		{"unmarshalable", &Output{Table: []Column{{Header: "X", Path: "a"}}}, func() {}, "render:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(context.Background(), tt.out, tt.result)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Render error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRenderStopsRunawayJQ(t *testing.T) {
	out := &Output{Table: []Column{{Header: "X", JQ: "def f: f; f"}}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := Render(ctx, out, []any{1}); err == nil {
		t.Fatal("expected an error for a non-terminating expression")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("Render did not honour the context deadline")
	}

	infinite := &Output{Table: []Column{{Header: "X", JQ: "repeat(1)"}}}
	got, err := Render(context.Background(), infinite, []any{1})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got.Rows[0][0], ","); n != maxCellResults-1 {
		t.Errorf("cell has %d results, want %d", n+1, maxCellResults)
	}
}

func TestFormatCell(t *testing.T) {
	tests := []struct {
		v    any
		want string
	}{
		{nil, ""},
		{"s", "s"},
		{json.Number("1.50"), "1.50"},
		{true, "true"},
		{3, "3"},
		{int64(-4), "-4"},
		{1e21, "1000000000000000000000"},
		{[]any{}, ""},
		{[]any{1, "a", nil}, "1, a, "},
		{[]any{[]any{1}}, "[[1]]"},
		{map[string]any{"b": 1, "a": 2}, `{"a":2,"b":1}`},
	}
	for _, tt := range tests {
		if got := formatCell(tt.v); got != tt.want {
			t.Errorf("formatCell(%#v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}
