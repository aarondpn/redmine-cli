package cmdutil

import (
	"testing"
	"time"
)

func TestResolveDateKeyword_Today(t *testing.T) {
	fixed := time.Date(2026, 4, 24, 10, 30, 0, 0, time.UTC)

	cases := []struct {
		name string
		in   string
	}{
		{"lowercase", "today"},
		{"uppercase", "TODAY"},
		{"mixed case", "ToDay"},
		{"padded", "  today  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDateKeywordAt(tc.in, fixed)
			if got != "2026-04-24" {
				t.Errorf("resolveDateKeywordAt(%q) = %q, want 2026-04-24", tc.in, got)
			}
		})
	}
}

func TestResolveDateKeyword_PassThrough(t *testing.T) {
	fixed := time.Date(2026, 4, 24, 10, 30, 0, 0, time.UTC)

	cases := []string{
		"",
		"2026-06-30",
		"not-a-date",
		"tomorrow", // only "today" is supported today; everything else is passed through
	}
	for _, in := range cases {
		got := resolveDateKeywordAt(in, fixed)
		if got != in {
			t.Errorf("resolveDateKeywordAt(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestResolveDateKeyword_UsesWallClock(t *testing.T) {
	// Smoke test for the public function: resolving "today" must equal
	// time.Now().Format(dateLayout) taken around the same instant.
	before := time.Now().Format("2006-01-02")
	got := ResolveDateKeyword("today")
	after := time.Now().Format("2006-01-02")
	if got != before && got != after {
		t.Errorf("ResolveDateKeyword(\"today\") = %q, want %q or %q", got, before, after)
	}
}
