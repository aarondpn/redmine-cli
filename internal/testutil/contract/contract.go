// Package contract compares the JSON Redmine returned for a resource with
// the JSON the CLI or the MCP server produced for the same resource, and
// reports every place where data was dropped or altered on the way.
//
// The comparison is driven by the raw payload: whatever Redmine sent decides
// what the output must contain, so the same test adapts to every Redmine
// version without any version knowledge. Deliberate differences (the CLI
// prints lists as bare arrays, MCP redacts api_key, ...) are declared as
// Rules, each with a mandatory reason; a rule that matches nothing is itself
// a failure so the allowlist cannot rot.
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// Kind classifies a finding.
type Kind string

const (
	// KindMissing: Redmine sent a non-zero value, the output has no such key.
	// This is the bug class issue #202 is about (a model without the field).
	KindMissing Kind = "MISSING"
	// KindZeroDrop: Redmine sent a zero value ("", 0, false, [], {}) and the
	// output has no such key, typically because of an omitempty tag.
	KindZeroDrop Kind = "ZERO-DROP"
	// KindValue: both sides have the key but the values differ.
	KindValue Kind = "VALUE"
	// KindExtra: the output has a non-zero value for a key Redmine did not
	// send. A zero value is tolerated because the CLI models emit defaults
	// such as "" or [] for absent keys.
	KindExtra Kind = "EXTRA"
	// KindLen: an array has a different number of elements.
	KindLen Kind = "ARRAY-LEN"
	// KindLeak: a key that a rule declares must be absent is present.
	KindLeak Kind = "LEAK"
)

// Rule declares a deliberate difference between the raw payload and the
// output.
//
// Path addresses a key by its dotted path from the root, with "[]" for any
// array element: "journals[].details[].name". A "*" segment matches exactly
// one key and a leading "**." matches any depth (only meant for secret
// redaction).
type Rule struct {
	Path   string
	Kind   Kind
	Reason string
	// Absent additionally asserts that no non-zero value is present at Path
	// in the output. Used for redacted secrets: it proves the redaction is
	// still active rather than the field merely being unmodelled.
	Absent bool
	// Optional exempts the rule from the "matched nothing" check. It is for
	// rules shared by many cases, where any single case may legitimately not
	// contain the key.
	Optional bool
}

// Finding is one difference.
type Finding struct {
	Path string
	Kind Kind
	Raw  string
	Got  string
}

// Result is the outcome of a Diff.
type Result struct {
	Findings []Finding
	// BadRules lists rules that matched nothing, or are invalid.
	BadRules []string
}

// OK reports whether the comparison found nothing to complain about.
func (r Result) OK() bool { return len(r.Findings) == 0 && len(r.BadRules) == 0 }

// String renders the result as a table, one finding per line.
func (r Result) String() string {
	var b strings.Builder
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "  %-9s %s", f.Kind, f.Path)
		if f.Raw != "" || f.Got != "" {
			fmt.Fprintf(&b, "   raw=%s got=%s", f.Raw, f.Got)
		}
		b.WriteByte('\n')
	}
	for _, u := range r.BadRules {
		fmt.Fprintf(&b, "  RULE      %s\n", u)
	}
	return b.String()
}

// Decode parses JSON preserving number literals, which Diff needs to compare
// 8 and 8.0 numerically without float rounding.
func Decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// Normalize round-trips an arbitrary value (for example a struct) through
// JSON so it can be handed to Diff.
func Normalize(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return Decode(data)
}

// Diff compares raw (what Redmine sent) with got (what the CLI or MCP
// produced). A raw null counts as an absent key.
func Diff(raw, got any, rules []Rule) Result {
	d := &differ{}
	d.walk(nil, raw, got)
	d.leaks(got, rules)

	var res Result
	used := make([]bool, len(rules))
	parsed := make([][]string, len(rules))
	for i, r := range rules {
		parsed[i] = splitPath(r.Path)
		if strings.TrimSpace(r.Reason) == "" {
			res.BadRules = append(res.BadRules, fmt.Sprintf("%s %s: rule has no reason", r.Kind, r.Path))
		}
	}
	for _, f := range d.findings {
		allowed := false
		for i, r := range rules {
			if r.Kind == f.Kind && matchPath(parsed[i], splitPath(f.Path)) {
				used[i] = true
				allowed = true
			}
		}
		if !allowed {
			res.Findings = append(res.Findings, f)
		}
	}
	for i, r := range rules {
		if !used[i] && !r.Optional {
			res.BadRules = append(res.BadRules, fmt.Sprintf("%s %s: rule matched nothing (stale: %s)", r.Kind, r.Path, r.Reason))
		}
	}
	sort.SliceStable(res.Findings, func(i, j int) bool { return res.Findings[i].Path < res.Findings[j].Path })
	return res
}

type differ struct {
	findings []Finding
}

func (d *differ) add(path []string, kind Kind, raw, got any) {
	d.findings = append(d.findings, Finding{Path: joinPath(path), Kind: kind, Raw: short(raw), Got: short(got)})
}

func child(path []string, seg string) []string {
	return append(path[:len(path):len(path)], seg)
}

func (d *differ) walk(path []string, raw, got any) {
	switch rv := raw.(type) {
	case map[string]any:
		gv, ok := got.(map[string]any)
		if !ok {
			d.add(path, KindValue, raw, got)
			return
		}
		for k, r := range rv {
			if r == nil {
				continue
			}
			g := gv[k]
			switch {
			case g == nil && isZero(r):
				d.add(child(path, k), KindZeroDrop, r, nil)
			case g == nil:
				d.add(child(path, k), KindMissing, r, nil)
			default:
				d.walk(child(path, k), r, g)
			}
		}
		for k, g := range gv {
			if rv[k] == nil && !isZero(g) {
				d.add(child(path, k), KindExtra, nil, g)
			}
		}
	case []any:
		gv, ok := got.([]any)
		if !ok {
			d.add(path, KindValue, raw, got)
			return
		}
		if len(rv) != len(gv) {
			d.add(path, KindLen, len(rv), len(gv))
		}
		for i := 0; i < len(rv) && i < len(gv); i++ {
			d.walk(child(path, "[]"), rv[i], gv[i])
		}
	case json.Number:
		gn, ok := got.(json.Number)
		if !ok || !sameNumber(rv, gn) {
			d.add(path, KindValue, raw, got)
		}
	default:
		if raw != got {
			d.add(path, KindValue, raw, got)
		}
	}
}

// leaks reports non-zero values in got at paths a rule marked Absent.
func (d *differ) leaks(got any, rules []Rule) {
	var patterns [][]string
	for _, r := range rules {
		if r.Absent {
			patterns = append(patterns, splitPath(r.Path))
		}
	}
	if len(patterns) == 0 {
		return
	}
	var visit func(path []string, v any)
	visit = func(path []string, v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, c := range t {
				p := child(path, k)
				for _, pat := range patterns {
					if matchPath(pat, p) && !isZero(c) {
						d.add(p, KindLeak, nil, c)
					}
				}
				visit(p, c)
			}
		case []any:
			for _, c := range t {
				visit(child(path, "[]"), c)
			}
		}
	}
	visit(nil, got)
}

func isZero(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case bool:
		return !t
	case json.Number:
		r, ok := new(big.Rat).SetString(t.String())
		return ok && r.Sign() == 0
	case []any:
		for _, e := range t {
			if !isZero(e) {
				return false
			}
		}
		return true
	case map[string]any:
		for _, e := range t {
			if !isZero(e) {
				return false
			}
		}
		return true
	}
	return false
}

func sameNumber(a, b json.Number) bool {
	ra, ok1 := new(big.Rat).SetString(a.String())
	rb, ok2 := new(big.Rat).SetString(b.String())
	if !ok1 || !ok2 {
		return a == b
	}
	return ra.Cmp(rb) == 0
}

func short(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	s := string(data)
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

func joinPath(segs []string) string {
	var b strings.Builder
	for _, s := range segs {
		if s == "[]" {
			b.WriteString("[]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s)
	}
	return b.String()
}

// splitPath turns "a[].b" into ["a", "[]", "b"].
func splitPath(p string) []string {
	p = strings.ReplaceAll(p, "[]", ".[].")
	var out []string
	for _, s := range strings.Split(p, ".") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Get returns the non-null values found at path in v ("[]" fans out over
// arrays). It backs the coverage assertions that a fixture really makes
// Redmine emit a version-gated key.
func Get(v any, path string) []any {
	var out []any
	var visit func(v any, segs []string)
	visit = func(v any, segs []string) {
		if v == nil {
			return
		}
		if len(segs) == 0 {
			out = append(out, v)
			return
		}
		switch t := v.(type) {
		case map[string]any:
			if segs[0] != "[]" {
				visit(t[segs[0]], segs[1:])
			}
		case []any:
			if segs[0] == "[]" {
				for _, e := range t {
					visit(e, segs[1:])
				}
			}
		}
	}
	visit(v, splitPath(path))
	return out
}

func matchPath(pattern, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(path); i++ {
			if matchPath(pattern[1:], path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	if pattern[0] != "*" && pattern[0] != path[0] {
		return false
	}
	return matchPath(pattern[1:], path[1:])
}
