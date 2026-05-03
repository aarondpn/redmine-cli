//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestAPI_Passthrough_GET exercises the GET path with query params.
func TestAPI_Passthrough_GET(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var resp struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	r.runJSON(t, &resp, "api", "/users/current.json")
	if resp.User.Login != e2eUsername() {
		t.Fatalf("api GET /users/current.json login = %q, want %q", resp.User.Login, e2eUsername())
	}
}

// TestAPI_Passthrough_POSTAndPUT creates an issue via raw POST with an input
// body file, then updates it via raw PUT. This covers the write-side of the
// passthrough that the GET test cannot.
func TestAPI_Passthrough_POSTAndPUT(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	// Look up project ID and first tracker so we can build a valid request.
	var projectInfo struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &projectInfo, "projects", "get", proj.Identifier)
	var trackers []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &trackers, "trackers", "list")
	if len(trackers) == 0 {
		t.Fatal("no trackers available")
	}

	// POST a new issue.
	subject := "Created via raw POST"
	createBody, _ := json.Marshal(map[string]any{
		"issue": map[string]any{
			"project_id": projectInfo.ID,
			"tracker_id": trackers[0].ID,
			"subject":    subject,
		},
	})
	createPath := writeBodyFile(t, createBody)

	var created struct {
		Issue struct {
			ID      int    `json:"id"`
			Subject string `json:"subject"`
		} `json:"issue"`
	}
	r.runJSON(t, &created, "api", "/issues.json", "-X", "POST", "--input", createPath)
	if created.Issue.ID == 0 {
		t.Fatalf("api POST returned no issue ID; got %+v", created)
	}
	if created.Issue.Subject != subject {
		t.Fatalf("api POST subject = %q, want %q", created.Issue.Subject, subject)
	}

	// PUT an update. Redmine returns 204 No Content on success, so we don't
	// parse the body; we just require a clean exit and verify via a GET.
	updateBody, _ := json.Marshal(map[string]any{
		"issue": map[string]any{
			"subject": "Updated via raw PUT",
		},
	})
	updatePath := writeBodyFile(t, updateBody)
	r.run(t, "api", fmt.Sprintf("/issues/%d.json", created.Issue.ID),
		"-X", "PUT", "--input", updatePath)

	var got struct {
		Issue struct {
			Subject string `json:"subject"`
		} `json:"issue"`
	}
	r.runJSON(t, &got, "api", fmt.Sprintf("/issues/%d.json", created.Issue.ID))
	if !strings.Contains(got.Issue.Subject, "Updated via raw PUT") {
		t.Fatalf("subject after PUT = %q, want contains %q", got.Issue.Subject, "Updated via raw PUT")
	}
}

func writeBodyFile(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	return path
}

// TestAPI_NotFoundResponse exercises the raw api passthrough against a
// non-existent issue. Distinct from TestErrors_NotFound (typed `issues get`)
// because it specifically covers the api passthrough's error-mapping layer.
func TestAPI_NotFoundResponse(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout, _ := r.runExpectError(t, "api", "/issues/2147483600.json")
	assertErrorCode(t, stdout, "not_found")
}

// TestAPI_Delete creates an issue via the typed CLI and removes it via the
// raw api passthrough (DELETE returns 204 No Content). The follow-up
// `issues get` must surface a not_found envelope.
func TestAPI_Delete(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	r.run(t, "api", fmt.Sprintf("/issues/%d.json", issue.ID), "-X", "DELETE")

	stdout, _ := r.runExpectError(t, "issues", "get", strconv.Itoa(issue.ID))
	assertErrorCode(t, stdout, "not_found")
}

// TestAPI_FormFlag exercises -f as query parameters on a GET endpoint by
// asking for at most 1 issue across all statuses; the response must contain
// no more than 1 entry in the `issues` array and echo limit=1.
func TestAPI_FormFlag(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	_ = createTestIssue(t, r, proj.Identifier)

	var resp struct {
		Issues []struct {
			ID int `json:"id"`
		} `json:"issues"`
		Limit int `json:"limit"`
	}
	r.runJSON(t, &resp, "api", "/issues.json", "-f", "limit=1", "-f", "status_id=*")

	if len(resp.Issues) > 1 {
		t.Fatalf("api -f limit=1 returned %d issues, want <= 1", len(resp.Issues))
	}
	if resp.Limit != 1 {
		t.Fatalf("api -f limit=1 echoed limit = %d, want 1", resp.Limit)
	}
}
