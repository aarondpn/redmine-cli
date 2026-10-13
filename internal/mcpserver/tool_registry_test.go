package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestEmptyNilSlices(t *testing.T) {
	type result struct {
		Items    []int    `json:"items"`
		Optional []string `json:"optional,omitzero"`
		Legacy   []string `json:"legacy,omitempty"`
		Hidden   []int    `json:"-"`
		Filled   []int    `json:"filled"`
		Count    int      `json:"count"`
	}

	v := result{Filled: []int{1}}
	emptyNilSlices(&v)

	if v.Items == nil {
		t.Error("plain slice: want empty slice, got nil")
	}
	if v.Optional != nil {
		t.Errorf("omitzero slice: want nil (omitted), got %#v", v.Optional)
	}
	if v.Legacy != nil {
		t.Errorf("omitempty slice: want nil (omitted), got %#v", v.Legacy)
	}
	if v.Hidden != nil {
		t.Errorf(`json:"-" slice: want nil, got %#v`, v.Hidden)
	}
	if len(v.Filled) != 1 {
		t.Errorf("non-nil slice must be untouched, got %#v", v.Filled)
	}

	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(out), `{"items":[],"filled":[1],"count":0}`; got != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}

func TestEmptyNilSlices_NonStruct(t *testing.T) {
	var s []int
	emptyNilSlices(&s) // must not panic; top-level slices are left as-is
	if s != nil {
		t.Errorf("top-level slice changed: %#v", s)
	}
	var p *struct {
		Items []int `json:"items"`
	}
	emptyNilSlices(&p) // nil pointer result must not panic
}
