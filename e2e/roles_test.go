//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

func TestRoles_ListAndGet(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	role := firstRole(t, r)

	var roles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &roles, "roles", "list")
	if len(roles) == 0 {
		t.Fatal("roles list returned no roles")
	}

	found := false
	for _, item := range roles {
		if item.ID == role.ID && item.Name == role.Name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("roles list did not include %+v", role)
	}

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
