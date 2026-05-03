//go:build e2e

package e2e

import (
	"encoding/json"
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

// TestSearch_Wiki creates a wiki page via the raw api passthrough (PUT
// /projects/<id>/wiki/<page>.json with a unique token in the body) and
// verifies the dedicated `search wiki` subcommand surfaces it.
func TestSearch_Wiki(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	token := fmt.Sprintf("wikitoken%d", time.Now().UnixNano())
	pageTitle := fmt.Sprintf("E2EWikiPage%d", time.Now().UnixNano())

	body, err := json.Marshal(map[string]any{
		"wiki_page": map[string]any{
			"text": "Wiki page created by e2e test " + token,
		},
	})
	if err != nil {
		t.Fatalf("marshal wiki body: %v", err)
	}
	bodyPath := writeBodyFile(t, body)

	endpoint := fmt.Sprintf("/projects/%s/wiki/%s.json", proj.Identifier, pageTitle)
	r.run(t, "api", endpoint, "-X", "PUT", "--input", bodyPath)

	var results []struct {
		Type string `json:"type"`
	}
	r.runJSON(t, &results, "search", "wiki", token, "--limit", "25")

	for _, res := range results {
		if strings.Contains(strings.ToLower(res.Type), "wiki") {
			return
		}
	}
	t.Fatalf("search wiki %q did not return any wiki-page result; got %+v", token, results)
}

// TestSearch_AllScopes runs `search <token>` with no scope flags and verifies
// the issue is returned. Redmine's default behavior, when no scope is set, is
// to search all resource types.
func TestSearch_AllScopes(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	token := fmt.Sprintf("allscopes%d", time.Now().UnixNano())
	issue := createTestIssueWithSubject(t, r, proj.Identifier, "Find all scopes "+token)

	var results []struct {
		ID   int    `json:"id"`
		Type string `json:"type"`
	}
	r.runJSON(t, &results, "search", token, "--limit", "25")

	for _, res := range results {
		if res.ID == issue.ID && strings.Contains(res.Type, "issue") {
			return
		}
	}
	t.Fatalf("default-scope search %q did not find issue %d; got %+v", token, issue.ID, results)
}

// TestSearch_NoResults verifies that a query guaranteed to match nothing
// returns an empty JSON array.
func TestSearch_NoResults(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout := r.run(t, "search", "xxxxxxxxxxnonexistenttoken12345", "--issues", "--limit", "5")
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Fatalf("expected empty JSON array, got: %s", got)
	}
}
