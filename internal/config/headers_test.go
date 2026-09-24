package config

import "testing"

func TestParseHeader(t *testing.T) {
	tests := []struct {
		in, name, value string
		wantErr         bool
	}{
		{in: "User-Agent: Mozilla/5.0 (X11; Linux x86_64)", name: "User-Agent", value: "Mozilla/5.0 (X11; Linux x86_64)"},
		{in: "X-Token:abc:def", name: "X-Token", value: "abc:def"},
		{in: "X-Empty:", name: "X-Empty", value: ""},
		{in: "no colon", wantErr: true},
		{in: ": value", wantErr: true},
		{in: "Bad Name: value", wantErr: true},
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

func TestMergeHeadersDoesNotMutateBase(t *testing.T) {
	base := map[string]string{"X-A": "1"}
	merged, err := MergeHeaders(base, []string{"x-a: 2", "X-B: 3"})
	if err != nil {
		t.Fatal(err)
	}
	if base["X-A"] != "1" || len(base) != 1 {
		t.Fatalf("base was mutated: %v", base)
	}
	if len(merged) != 2 || merged["x-a"] != "2" || merged["X-B"] != "3" {
		t.Fatalf("merged = %v", merged)
	}
}
