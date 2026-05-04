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
