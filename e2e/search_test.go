//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestSearch_Issues creates an issue with a unique token in its subject, then
// verifies the search command finds it.
func TestSearch_Issues(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	token := fmt.Sprintf("etoken%d", time.Now().UnixNano())
	subject := "Find me via search " + token
	issue := createTestIssueWithSubject(t, r, proj.Identifier, subject)

	var results []struct {
		ID    int    `json:"id"`
		Type  string `json:"type"`
		Title string `json:"title"`
	}
	r.runJSON(t, &results, "search", token, "--issues", "--limit", "25")

	found := false
	for _, res := range results {
		if strings.Contains(res.Type, "issue") && res.ID == issue.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("search %q did not find issue %d; got %+v", token, issue.ID, results)
	}
}

// TestSearch_Projects verifies the --projects scope hits the project index.
func TestSearch_Projects(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	// Query by the project identifier's trailing nanosecond suffix, which is
	// unique per run and unlikely to match any other project.
	parts := strings.Split(proj.Identifier, "-")
	token := parts[len(parts)-1]

	var results []struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	}
	r.runJSON(t, &results, "search", token, "--projects", "--limit", "25")

	found := false
	for _, res := range results {
		if res.Type == "project" && strings.Contains(res.Title, token) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("search %q --projects did not return %q; got %+v", token, proj.Identifier, results)
	}
}
