package config

import "testing"

func TestParseHeader(t *testing.T) {
	tests := []struct {
		in, name, value string
		wantErr         bool
	}{
		{in: "User-Agent: Mozilla/5.0 (X11; Linux x86_64)", name: "User-Agent", value: "Mozilla/5.0 (X11; Linux x86_64)"},
		{in: "x-token:abc:def", name: "X-Token", value: "abc:def"},
		{in: "no colon", wantErr: true},
		{in: ": value", wantErr: true},
		{in: "Bad Name: value", wantErr: true},
		{in: "X-Empty:", wantErr: true},
		{in: "X-Evil: a\r\nX-Injected: b", wantErr: true},
		{in: "Host: example.com", wantErr: true},
		{in: "content-type: text/plain", wantErr: true},
		{in: "Accept-Encoding: gzip", wantErr: true},
	}
	for _, tt := range tests {
		name, value, err := ParseHeader(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseHeader(%q) expected error", tt.in)
			}
			continue
		}
		if err != nil || name != tt.name || value != tt.value {
			t.Errorf("ParseHeader(%q) = %q, %q, %v; want %q, %q", tt.in, name, value, err, tt.name, tt.value)
		}
	}
}

func TestMergeHeadersCanonicalizesAndDoesNotMutateBase(t *testing.T) {
	base := map[string]string{"x-a": "1"}
	merged, err := MergeHeaders(base, []string{"X-A: 2", "x-b: 3"})
	if err != nil {
		t.Fatal(err)
	}
	if base["x-a"] != "1" || len(base) != 1 {
		t.Fatalf("base was mutated: %v", base)
	}
	if len(merged) != 2 || merged["X-A"] != "2" || merged["X-B"] != "3" {
		t.Fatalf("merged = %v", merged)
	}
}

func TestMergeHeadersValidatesBase(t *testing.T) {
	for name, base := range map[string]map[string]string{
		"case-duplicate": {"User-Agent": "a", "user-agent": "b"},
		"reserved":       {"Content-Length": "1"},
		"empty value":    {"X-A": ""},
		"invalid value":  {"X-A": "a\nb"},
	} {
		if _, err := MergeHeaders(base, nil); err == nil {
			t.Errorf("%s: expected error for %v", name, base)
		}
	}
}

func TestSameServerHost(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://redmine.example.com", "https://Redmine.example.com/", true},
		{"https://redmine.example.com", "redmine.example.com", true},
		{"https://redmine.example.com", "https://other.example.com", false},
		{"https://redmine.example.com:8443", "https://redmine.example.com", false},
		{"", "", false},
	}
	for _, tt := range tests {
		if got := SameServerHost(tt.a, tt.b); got != tt.want {
			t.Errorf("SameServerHost(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
