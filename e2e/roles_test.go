//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

// TestRoles_ListAndGet checks the table rendering of `roles get`. The JSON
// payloads of `roles list` and `roles get` are asserted by the contract tests.
func TestRoles_ListAndGet(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	role := firstRole(t, r)

	stdout := r.run(t, "roles", "get", strconv.Itoa(role.ID), "--output", "table")
	text := string(stdout)
	if !strings.Contains(text, role.Name) {
		t.Fatalf("roles get output missing %q\nstdout:\n%s", role.Name, stdout)
	}
}

// TestRoles_ListHasNoBuiltinColumns guards against phantom builtin/assignable
// fields: /roles.json only returns id and name.
func TestRoles_ListHasNoBuiltinColumns(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	table := string(r.run(t, "roles", "list", "--output", "table"))
	if !strings.Contains(table, "Name") {
		t.Fatalf("roles table missing Name column\n%s", table)
	}
	for _, col := range []string{"Builtin", "Assignable"} {
		if strings.Contains(table, col) {
			t.Errorf("roles table has phantom %q column\n%s", col, table)
		}
	}
}
