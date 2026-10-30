package contract

import (
	"strings"
	"testing"
)

func mustDecode(t *testing.T, s string) any {
	t.Helper()
	v, err := Decode([]byte(s))
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return v
}

func kinds(r Result) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, string(f.Kind)+" "+f.Path)
	}
	return out
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		got   string
		rules []Rule
		want  []string
	}{
		{name: "identical", raw: `{"a":1,"b":"x","c":[{"d":true}]}`, got: `{"a":1,"b":"x","c":[{"d":true}]}`},
		{name: "numbers compare numerically", raw: `{"a":8.0,"b":0.50}`, got: `{"a":8,"b":0.5}`},
		{name: "string is not a number", raw: `{"a":5}`, got: `{"a":"5"}`, want: []string{"VALUE a"}},
		{name: "missing key", raw: `{"a":{"is_closed":true,"id":1}}`, got: `{"a":{"id":1}}`, want: []string{"MISSING a.is_closed"}},
		{name: "zero value dropped", raw: `{"a":false,"b":"","c":0,"d":[]}`, got: `{}`,
			want: []string{"ZERO-DROP a", "ZERO-DROP b", "ZERO-DROP c", "ZERO-DROP d"}},
		{name: "raw null equals absent", raw: `{"a":null}`, got: `{}`},
		{name: "null in output counts as absent", raw: `{"a":"x"}`, got: `{"a":null}`, want: []string{"MISSING a"}},
		{name: "extra zero tolerated", raw: `{}`, got: `{"children":[],"description":"","n":0}`},
		{name: "extra non-zero fails", raw: `{}`, got: `{"invented":"x"}`, want: []string{"EXTRA invented"}},
		{name: "array length", raw: `{"a":[1,2,3]}`, got: `{"a":[1,2]}`, want: []string{"ARRAY-LEN a"}},
		{name: "nested array element path", raw: `{"j":[{"d":[{"name":"x"}]}]}`, got: `{"j":[{"d":[{}]}]}`, want: []string{"MISSING j[].d[].name"}},
		{name: "type mismatch", raw: `{"a":{"b":1}}`, got: `{"a":[]}`, want: []string{"VALUE a"}},
		{name: "key order and extra array", raw: `[{"b":1,"a":2}]`, got: `[{"a":2,"b":1}]`},
		{name: "allowed missing", raw: `{"total_count":3,"issues":[]}`, got: `{"issues":[]}`,
			rules: []Rule{{Path: "total_count", Kind: KindMissing, Reason: "bare array"}}},
		{name: "wildcard segment", raw: `{"a":{"x":1,"y":2}}`, got: `{"a":{}}`,
			rules: []Rule{{Path: "a.*", Kind: KindMissing, Reason: "r"}}},
		{name: "any depth", raw: `{"u":{"api_key":"k"},"l":[{"api_key":"k"}]}`, got: `{"u":{},"l":[{}]}`,
			rules: []Rule{{Path: "**.api_key", Kind: KindMissing, Reason: "redacted", Absent: true}}},
		{name: "redaction leak", raw: `{"api_key":"k"}`, got: `{"api_key":"k"}`,
			rules: []Rule{{Path: "**.api_key", Kind: KindMissing, Reason: "redacted", Absent: true}},
			want:  []string{"LEAK api_key"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := Diff(mustDecode(t, tc.raw), mustDecode(t, tc.got), tc.rules)
			got := kinds(res)
			wantBad := 0
			for _, r := range tc.rules {
				if r.Absent {
					wantBad++ // the leak-check rules legitimately match nothing when redaction works
				}
			}
			if len(res.BadRules) > wantBad {
				t.Fatalf("unexpected bad rules: %v", res.BadRules)
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("findings = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDiffRuleHygiene(t *testing.T) {
	raw := mustDecode(t, `{"a":1}`)
	got := mustDecode(t, `{"a":1}`)

	res := Diff(raw, got, []Rule{{Path: "gone", Kind: KindMissing, Reason: "stale"}})
	if res.OK() || len(res.BadRules) != 1 || !strings.Contains(res.BadRules[0], "matched nothing") {
		t.Fatalf("stale rule must fail: %+v", res)
	}

	res = Diff(mustDecode(t, `{"a":1}`), mustDecode(t, `{}`), []Rule{{Path: "a", Kind: KindMissing}})
	if res.OK() || len(res.BadRules) != 1 || !strings.Contains(res.BadRules[0], "no reason") {
		t.Fatalf("rule without reason must fail: %+v", res)
	}

	res = Diff(raw, got, []Rule{{Path: "gone", Kind: KindMissing, Reason: "shared", Optional: true}})
	if !res.OK() {
		t.Fatalf("optional rule must not be reported stale: %+v", res)
	}

	if res := Diff(raw, got, nil); !res.OK() {
		t.Fatalf("clean diff must be OK: %+v", res)
	}
}

func TestGet(t *testing.T) {
	v := mustDecode(t, `{"j":[{"u":{"id":1}},{"u":null},{}],"s":"x"}`)
	if got := Get(v, "j[].u.id"); len(got) != 1 {
		t.Fatalf("Get j[].u.id = %v, want one value", got)
	}
	if got := Get(v, "s"); len(got) != 1 {
		t.Fatalf("Get s = %v", got)
	}
	if got := Get(v, "j[].missing"); len(got) != 0 {
		t.Fatalf("Get missing = %v", got)
	}
}
