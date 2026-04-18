package mcpserver

import (
	"testing"
)

func TestParseRedmineURI(t *testing.T) {
	cases := []struct {
		name     string
		uri      string
		wantKind string
		wantSegs []string
		wantErr  bool
	}{
		{"issue", "redmine://issue/42", "issue", []string{"42"}, false},
		{"project identifier", "redmine://project/foo-bar", "project", []string{"foo-bar"}, false},
		{"user me", "redmine://user/me", "user", []string{"me"}, false},
		{"wiki two segments", "redmine://wiki/proj/Getting%20Started", "wiki", []string{"proj", "Getting Started"}, false},
		{"missing kind", "redmine:///123", "", nil, true},
		{"wrong scheme", "http://issue/1", "", nil, true},
		{"trailing slash", "redmine://issue/", "issue", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, parts, err := parseRedmineURI(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got kind=%q parts=%v", kind, parts)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", kind, tc.wantKind)
			}
			if !equalStringSlices(parts, tc.wantSegs) {
				t.Errorf("parts = %v, want %v", parts, tc.wantSegs)
			}
		})
	}
}

func TestParseIntID(t *testing.T) {
	if _, err := parseIntID("abc"); err == nil {
		t.Error("expected error for non-numeric segment")
	}
	if _, err := parseIntID("0"); err == nil {
		t.Error("expected error for zero ID")
	}
	if _, err := parseIntID("-5"); err == nil {
		t.Error("expected error for negative ID")
	}
	id, err := parseIntID("17")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 17 {
		t.Errorf("id = %d, want 17", id)
	}
}

func TestExpectSingleSegment(t *testing.T) {
	if _, err := expectSingleSegment(nil, "issue"); err == nil {
		t.Error("expected error for empty slice")
	}
	if _, err := expectSingleSegment([]string{"a", "b"}, "issue"); err == nil {
		t.Error("expected error for multi-segment")
	}
	if _, err := expectSingleSegment([]string{""}, "issue"); err == nil {
		t.Error("expected error for empty string segment")
	}
	s, err := expectSingleSegment([]string{"ok"}, "issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "ok" {
		t.Errorf("got %q, want ok", s)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
