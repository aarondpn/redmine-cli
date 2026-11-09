package models

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestFilterValues_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		in   string
		want FilterValues
	}{
		{`">=2025-01-01"`, FilterValues{">=2025-01-01"}},
		{`""`, FilterValues{""}},
		{`["status_id","created_on"]`, FilterValues{"status_id", "created_on"}},
		{`[]`, FilterValues{}},
	}
	for _, tt := range tests {
		var got FilterValues
		if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", tt.in, err)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("Unmarshal(%s) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func TestFilterValues_UnmarshalJSONRejectsOtherTypes(t *testing.T) {
	for _, in := range []string{`5`, `true`, `null`, `{"a":"b"}`, `[1,2]`} {
		var got FilterValues
		if err := json.Unmarshal([]byte(in), &got); err == nil {
			t.Errorf("Unmarshal(%s) = %#v, want error", in, got)
		}
	}
}

func TestFilterValues_MarshalJSON(t *testing.T) {
	tests := []struct {
		in   FilterValues
		want string
	}{
		{FilterValues{"a"}, `"a"`},
		{FilterValues{"a", "b"}, `["a","b"]`},
		{FilterValues{}, `[]`},
		{nil, `[]`},
	}
	for _, tt := range tests {
		got, err := json.Marshal(tt.in)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Errorf("Marshal(%#v) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestFilterParams_RoundTrip(t *testing.T) {
	in := `{"cf_5":"Critical","f[]":["status_id","created_on"]}`
	var p FilterParams
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p["cf_5"], FilterValues{"Critical"}) || !slices.Equal(p["f[]"], FilterValues{"status_id", "created_on"}) {
		t.Fatalf("decoded %#v", p)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Errorf("Marshal = %s, want %s", out, in)
	}
}
