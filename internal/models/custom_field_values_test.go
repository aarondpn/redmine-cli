package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCustomFieldInput_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   CustomFieldInput
		want string
	}{
		{"nil", nil, `""`},
		{"empty", CustomFieldInput{}, `""`},
		{"single empty", CustomFieldInput{""}, `""`},
		{"single", CustomFieldInput{"a"}, `"a"`},
		{"spaces", CustomFieldInput{"a b"}, `"a b"`},
		{"equals sign", CustomFieldInput{"x=y"}, `"x=y"`},
		{"two", CustomFieldInput{"a", "b"}, `["a","b"]`},
		{"order kept", CustomFieldInput{"b", "a"}, `["b","a"]`},
		{"leading empty dropped", CustomFieldInput{"", "a"}, `"a"`},
		{"inner empty dropped", CustomFieldInput{"a", "", "b"}, `["a","b"]`},
		{"only empties", CustomFieldInput{"", ""}, `""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCustomFieldInput_UnmarshalJSON(t *testing.T) {
	ok := []struct {
		in   string
		want CustomFieldInput
	}{
		{`"a"`, CustomFieldInput{"a"}},
		{`""`, CustomFieldInput{""}},
		{`["a","b"]`, CustomFieldInput{"a", "b"}},
		{`[]`, CustomFieldInput{}},
	}
	for _, tt := range ok {
		var got CustomFieldInput
		if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
			t.Errorf("%s: %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %#v, want %#v", tt.in, got, tt.want)
		}
	}

	var empty CustomFieldInput
	if err := json.Unmarshal([]byte(`[]`), &empty); err != nil {
		t.Fatal(err)
	}
	if out, _ := json.Marshal(empty); string(out) != `""` {
		t.Errorf("empty array marshals to %s, want \"\"", out)
	}

	for _, in := range []string{`3`, `true`, `null`, `{}`, `[1]`} {
		var got CustomFieldInput
		err := json.Unmarshal([]byte(in), &got)
		if err == nil {
			t.Errorf("%s: expected error", in)
			continue
		}
		if !strings.Contains(err.Error(), "string or an array of strings") {
			t.Errorf("%s: unexpected error %v", in, err)
		}
	}
}

func TestCustomFieldValues_InStructs(t *testing.T) {
	cfv := CustomFieldValues{"5": {"a"}, "6": {"x", "y"}, "7": {""}}
	data, err := json.Marshal(IssueCreate{ProjectID: 1, Subject: "s", CustomFieldValues: cfv})
	if err != nil {
		t.Fatal(err)
	}
	if want := `"custom_field_values":{"5":"a","6":["x","y"],"7":""}`; !strings.Contains(string(data), want) {
		t.Errorf("body %s lacks %s", data, want)
	}

	for name, v := range map[string]any{
		"IssueCreate":   IssueCreate{ProjectID: 1, Subject: "s"},
		"IssueUpdate":   IssueUpdate{},
		"ProjectCreate": ProjectCreate{Name: "n", Identifier: "i"},
		"ProjectUpdate": ProjectUpdate{},
		"empty map":     IssueCreate{ProjectID: 1, Subject: "s", CustomFieldValues: CustomFieldValues{}},
	} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(string(data), "custom_field_values") {
			t.Errorf("%s: body %s should omit custom_field_values", name, data)
		}
	}
}

func TestCustomFieldValues_RoundTrip(t *testing.T) {
	const in = `{"5":"a","6":["x","y"]}`
	var v CustomFieldValues
	if err := json.Unmarshal([]byte(in), &v); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Errorf("round trip = %s, want %s", out, in)
	}
}

func TestCustomFieldValues_InResourceStructs(t *testing.T) {
	cfv := CustomFieldValues{"1": {"a", "b"}}
	for name, v := range map[string]any{
		"TimeEntryCreate": TimeEntryCreate{Hours: 1, CustomFieldValues: cfv},
		"TimeEntryUpdate": TimeEntryUpdate{CustomFieldValues: cfv},
		"VersionCreate":   VersionCreate{Name: "v", CustomFieldValues: cfv},
		"VersionUpdate":   VersionUpdate{CustomFieldValues: cfv},
		"UserCreate":      UserCreate{Login: "l", CustomFieldValues: cfv},
		"UserUpdate":      UserUpdate{CustomFieldValues: cfv},
		"MyAccountUpdate": MyAccountUpdate{CustomFieldValues: cfv},
		"GroupCreate":     GroupCreate{Name: "g", CustomFieldValues: cfv},
		"GroupUpdate":     GroupUpdate{CustomFieldValues: cfv},
	} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := `"custom_field_values":{"1":["a","b"]}`; !strings.Contains(string(data), want) {
			t.Errorf("%s: body %s lacks %s", name, data, want)
		}
	}

	for name, v := range map[string]any{
		"TimeEntryCreate": TimeEntryCreate{Hours: 1},
		"TimeEntryUpdate": TimeEntryUpdate{},
		"VersionCreate":   VersionCreate{Name: "v"},
		"VersionUpdate":   VersionUpdate{},
		"UserCreate":      UserCreate{Login: "l"},
		"UserUpdate":      UserUpdate{},
		"MyAccountUpdate": MyAccountUpdate{},
		"GroupCreate":     GroupCreate{Name: "g"},
		"GroupUpdate":     GroupUpdate{},
	} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(string(data), "custom_field_values") {
			t.Errorf("%s: body %s should omit custom_field_values", name, data)
		}
	}
}
