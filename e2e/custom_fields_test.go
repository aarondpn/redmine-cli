//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

const seededCustomFieldName = "E2E Severity"

type e2eCustomField struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
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

// TestCustomFields_ListAndGet covers get by ID and by name and the table
// rendering. The JSON metadata (customized_type, field_format, possible_values,
// ...) is asserted by the contract tests.
func TestCustomFields_ListAndGet(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var fields []e2eCustomField
	r.runJSON(t, &fields, "custom-fields", "list")

	seeded := findCustomField(t, fields, seededCustomFieldName)

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

// TestCustomFields_Redmine7_ForAllProjectsRow checks the table rendering of the
// is_for_all flag Redmine 7.0 added (#44153). That the JSON carries is_for_all,
// projects and roles is asserted by the contract tests, which take the
// releases from the gate table in contract_gates_test.go.
func TestCustomFields_Redmine7_ForAllProjectsRow(t *testing.T) {
	requireE2E(t)
	skipIfVersionUnknown(t, "custom field scope metadata")
	skipBelowRedmine(t, 7, 0, "custom field scope metadata")

	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	stdout := r.run(t, "custom-fields", "get", seededCustomFieldName, "--output", "table")
	if !strings.Contains(string(stdout), "For All Projects") {
		t.Errorf("custom-fields get table output missing the For All Projects row\nstdout:\n%s", stdout)
	}
}
