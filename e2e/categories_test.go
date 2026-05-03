//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCategories_List creates two issue categories via the raw api passthrough
// (the categories cobra command only exposes `list`) and verifies that
// `categories list --project` returns both. Cleanup happens transitively when
// the parent project is deleted by createTestProject's t.Cleanup.
func TestCategories_List(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	createCategory(t, r, proj.Identifier, "E2E Cat 1")
	createCategory(t, r, proj.Identifier, "E2E Cat 2")

	var listed []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &listed, "categories", "list", "--project", proj.Identifier)

	if !containsCategoryName(listed, "E2E Cat 1") {
		t.Fatalf("categories list missing %q; got %+v", "E2E Cat 1", listed)
	}
	if !containsCategoryName(listed, "E2E Cat 2") {
		t.Fatalf("categories list missing %q; got %+v", "E2E Cat 2", listed)
	}
}

// TestCategories_List_UnknownProject verifies the CLI surfaces a non-zero exit
// and a structured error envelope when the target project does not exist. The
// project resolver intercepts the lookup before it reaches the API and emits
// a non-API "no match found" error (code "unknown"), so we assert on the
// envelope shape and the project identifier in the message rather than the
// code itself.
func TestCategories_List_UnknownProject(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	const missing = "nonexistent-project-12345"
	stdout, _ := r.runExpectError(t, "categories", "list", "--project", missing)

	var env errorEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstdout:\n%s", err, stdout)
	}
	if env.Error.Message == "" {
		t.Fatalf("error envelope missing message\nstdout:\n%s", stdout)
	}
	if !strings.Contains(env.Error.Message, missing) {
		t.Fatalf("error message %q does not mention %q\nstdout:\n%s",
			env.Error.Message, missing, stdout)
	}
}

func createCategory(t *testing.T, r *cliRunner, projectIdentifier, name string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"issue_category": map[string]any{"name": name},
	})
	if err != nil {
		t.Fatalf("marshal category body: %v", err)
	}
	bodyPath := writeBodyFile(t, body)
	r.run(t, "api", "/projects/"+projectIdentifier+"/issue_categories.json",
		"-X", "POST", "--input", bodyPath)
}

func containsCategoryName(items []struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}, name string) bool {
	for _, c := range items {
		if c.Name == name {
			return true
		}
	}
	return false
}
