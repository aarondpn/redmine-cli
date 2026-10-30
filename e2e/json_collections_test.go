//go:build e2e

package e2e

import (
	"encoding/json"
	"strconv"
	"testing"
)

// runJSONKeys runs a command in JSON mode and returns its top-level keys with
// their raw values, so tests can tell an absent key from an empty array.
func runJSONKeys(t *testing.T, r *cliRunner, args ...string) map[string]json.RawMessage {
	t.Helper()
	var keys map[string]json.RawMessage
	r.runJSON(t, &keys, args...)
	return keys
}

func requireEmptyArray(t *testing.T, keys map[string]json.RawMessage, key string) {
	t.Helper()
	raw, ok := keys[key]
	if !ok {
		t.Errorf("%q missing, want []", key)
		return
	}
	if string(raw) != "[]" {
		t.Errorf("%q = %s, want []", key, raw)
	}
}

func requireAbsent(t *testing.T, keys map[string]json.RawMessage, key string) {
	t.Helper()
	if raw, ok := keys[key]; ok {
		t.Errorf("%q = %s, want key absent", key, raw)
	}
}

// TestJSON_IssueGetEmptyIncludedCollections pins the contract agents rely on
// (`jq '.journals[]'`): requested-but-empty collections are [], unrequested
// ones are absent. A fresh issue is a leaf with no relations, which Redmine
// omits from the response even when included.
func TestJSON_IssueGetEmptyIncludedCollections(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	id := strconv.Itoa(issue.ID)

	included := runJSONKeys(t, r, "issues", "get", id,
		"--journals", "--children", "--relations", "--attachments")
	for _, key := range []string{"journals", "children", "relations", "attachments"} {
		requireEmptyArray(t, included, key)
	}
	requireAbsent(t, included, "watchers")

	plain := runJSONKeys(t, r, "issues", "get", id)
	for _, key := range []string{"journals", "children", "relations", "attachments", "watchers"} {
		requireAbsent(t, plain, key)
	}
}

func TestJSON_ProjectGetEmptyIncludedCollections(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	included := runJSONKeys(t, r, "projects", "get", proj.Identifier, "--include", "issue_categories")
	requireEmptyArray(t, included, "issue_categories")
	requireAbsent(t, included, "trackers")

	plain := runJSONKeys(t, r, "projects", "get", proj.Identifier)
	requireAbsent(t, plain, "issue_categories")
}

func TestJSON_GroupGetEmptyIncludedCollections(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	group := createTestGroup(t, r)
	id := strconv.Itoa(group.ID)

	included := runJSONKeys(t, r, "groups", "get", id, "--include", "users,memberships")
	requireEmptyArray(t, included, "users")
	requireEmptyArray(t, included, "memberships")

	plain := runJSONKeys(t, r, "groups", "get", id)
	requireAbsent(t, plain, "users")
	requireAbsent(t, plain, "memberships")
}
