//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

const (
	seededCustomFieldName          = "E2E Severity"
	seededTimeEntryCustomFieldName = "E2E Billing Code"
)

type e2eCustomField struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	CustomizedType string `json:"customized_type"`
	FieldFormat    string `json:"field_format"`
	IsForAll       *bool  `json:"is_for_all"`
	PossibleValues []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"possible_values"`
	Projects []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"projects"`
	Roles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"roles"`
}

// findCustomField returns the field with the given name, failing the test when
// the seeded fixture is missing.
func findCustomField(t *testing.T, fields []e2eCustomField, name string) e2eCustomField {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("custom-fields list missing seeded fixture %q; got %+v", name, fields)
	return e2eCustomField{}
}

func TestCustomFields_ListAndGet(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var fields []e2eCustomField
	r.runJSON(t, &fields, "custom-fields", "list")

	seeded := findCustomField(t, fields, seededCustomFieldName)
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

// TestCustomFields_Redmine7Metadata covers the two custom-field API changes in
// Redmine 7.0: is_for_all plus the associated projects on issue custom fields
// (#44153), and roles on non-issue custom fields (#44152).
func TestCustomFields_Redmine7Metadata(t *testing.T) {
	requireE2E(t)
	skipBelowRedmine(t, 7, 0, "custom field scope and role metadata")

	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var fields []e2eCustomField
	r.runJSON(t, &fields, "custom-fields", "list")

	// The bootstrap seeds the issue field with is_for_all, so the flag must be
	// present and true, and the projects array must stay empty: a field that
	// applies everywhere has no explicit project scope.
	issueField := findCustomField(t, fields, seededCustomFieldName)
	if issueField.IsForAll == nil {
		t.Errorf("issue custom field is missing is_for_all on Redmine %s: %+v", e2eVersion(), issueField)
	} else if !*issueField.IsForAll {
		t.Errorf("issue custom field is_for_all = false, want true (bootstrap seeds it for all projects)")
	}
	if len(issueField.Projects) != 0 {
		t.Errorf("issue custom field has %d projects, want 0 for an is_for_all field", len(issueField.Projects))
	}

	timeEntryField := findCustomField(t, fields, seededTimeEntryCustomFieldName)
	if len(timeEntryField.Roles) == 0 {
		t.Errorf("time entry custom field returned no roles on Redmine %s; #44152 should expose them: %+v",
			e2eVersion(), timeEntryField)
	}

	stdout := r.run(t, "custom-fields", "get", seededCustomFieldName, "--output", "table")
	if !strings.Contains(string(stdout), "For All Projects") {
		t.Errorf("custom-fields get table output missing the For All Projects row\nstdout:\n%s", stdout)
	}
}
