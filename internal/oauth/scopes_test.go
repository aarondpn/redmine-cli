package oauth

import (
	"slices"
	"strings"
	"testing"
)

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		readOnly bool
		want     []string
		wantErr  bool
	}{
		{"commas", "view_issues,log_time", false, []string{"view_issues", "log_time"}, false},
		{"spaces", "view_issues log_time", false, []string{"view_issues", "log_time"}, false},
		{"mixed", " view_issues, log_time  add_issues ,", false, []string{"view_issues", "log_time", "add_issues"}, false},
		{"default", "default", false, DefaultScopes(false), false},
		{"default read only", "default", true, DefaultScopes(true), false},
		{"default plus admin", "default,admin", false, append(DefaultScopes(false), "admin"), false},
		{"dedupe keeps order", "log_time,view_issues,log_time", false, []string{"log_time", "view_issues"}, false},
		{"default overlap deduped", "view_issues,default", true, append([]string{"view_issues"}, slices.DeleteFunc(DefaultScopes(true), func(s string) bool { return s == "view_issues" })...), false},
		{"uppercase", "View_Issues", false, nil, true},
		{"invalid chars", "view-issues", false, nil, true},
		{"digit", "view2", false, nil, true},
		{"empty", "", false, nil, true},
		{"only separators", " , ,", false, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScopes(tt.in, tt.readOnly)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseScopes(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ParseScopes(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestDefaultScopes(t *testing.T) {
	ro := DefaultScopes(true)
	rw := DefaultScopes(false)
	for _, s := range ro {
		if !strings.HasPrefix(s, "view_") && !strings.HasPrefix(s, "search_") {
			t.Errorf("read-only default contains %q", s)
		}
	}
	for _, s := range readScopes {
		if !slices.Contains(rw, s) {
			t.Errorf("default scopes lack %q", s)
		}
	}
	if len(rw) <= len(ro) {
		t.Errorf("write scopes missing: %d vs %d", len(rw), len(ro))
	}
	for _, set := range [][]string{ro, rw} {
		if slices.Contains(set, "admin") {
			t.Error("default scopes contain admin")
		}
	}
	// The result is a copy: callers may append without touching the package state.
	ro[0] = "changed"
	if readScopes[0] == "changed" {
		t.Error("DefaultScopes returned the shared slice")
	}
}
