//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

const seededCustomFieldName = "E2E Severity"

type e2eCustomField struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	CustomizedType string `json:"customized_type"`
	FieldFormat    string `json:"field_format"`
	PossibleValues []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"possible_values"`
}

func TestCustomFields_ListAndGet(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var fields []e2eCustomField
	r.runJSON(t, &fields, "custom-fields", "list")

	var seeded e2eCustomField
	for _, f := range fields {
		if f.Name == seededCustomFieldName {
			seeded = f
			break
		}
	}
	if seeded.ID == 0 {
		t.Fatalf("custom-fields list missing seeded fixture %q; got %+v", seededCustomFieldName, fields)
	}
	if seeded.CustomizedType != "issue" || seeded.FieldFormat != "list" {
		t.Fatalf("seeded custom field metadata unexpected: %+v", seeded)
	}
	if len(seeded.PossibleValues) == 0 {
		t.Fatalf("seeded custom field has no possible values: %+v", seeded)
	}

	var got e2eCustomField
	r.runJSON(t, &got, "custom-fields", "get", strconv.Itoa(seeded.ID))
	if got.ID != seeded.ID || got.Name != seeded.Name {
		t.Fatalf("custom-fields get returned %+v, want id=%d name=%q", got, seeded.ID, seeded.Name)
	}

	stdout := r.run(t, "custom-fields", "get", seeded.Name, "--output", "table")
	text := string(stdout)
	for _, want := range []string{seeded.Name, "Format", "list", "Possible Values"} {
		if !strings.Contains(text, want) {
			t.Fatalf("custom-fields get table output missing %q\nstdout:\n%s", want, stdout)
		}
	}
}
