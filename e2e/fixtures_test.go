//go:build e2e

package e2e

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

// projectFixture is an ephemeral test project. The underlying project is
// deleted by a t.Cleanup registered in createTestProject, so tests never
// share state.
type projectFixture struct {
	ID         int
	Name       string
	Identifier string
}

// createTestProject creates a uniquely-named project for a single test and
// registers its deletion via t.Cleanup. Use this for any test that needs
// write access to a project.
func createTestProject(t *testing.T, r *cliRunner) *projectFixture {
	t.Helper()
	identifier := uniqueIdentifier(t)
	name := "CLI E2E " + identifier

	var created struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Identifier string `json:"identifier"`
	}
	r.runJSON(t, &created, "projects", "create",
		"--name", name,
		"--identifier", identifier,
		"--description", "Created by redmine-cli e2e tests")
	if created.Identifier != identifier {
		t.Fatalf("created project identifier = %q, want %q", created.Identifier, identifier)
	}

	t.Cleanup(func() {
		var deleted actionEnvelope
		r.runJSON(t, &deleted, "projects", "delete", identifier, "--force")
		if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "project" {
			t.Errorf("unexpected project delete envelope: %+v", deleted)
		}
	})

	return &projectFixture{ID: created.ID, Name: created.Name, Identifier: created.Identifier}
}

// issueFixture holds the minimal data tests need about a created issue.
// The parent project's cleanup deletes the issue, so there is no separate
// issue cleanup.
type issueFixture struct {
	ID      int
	Subject string
}

// createTestIssue creates an issue in the given project using the first
// tracker available on the server.
func createTestIssue(t *testing.T, r *cliRunner, projectIdentifier string) *issueFixture {
	t.Helper()
	return createTestIssueWithSubject(t, r, projectIdentifier, "E2E issue "+time.Now().Format("20060102T150405.000"))
}

func createTestIssueWithSubject(t *testing.T, r *cliRunner, projectIdentifier, subject string) *issueFixture {
	t.Helper()
	return createTestIssueWithTracker(t, r, projectIdentifier, firstTrackerName(t, r), subject)
}

func createTestIssueWithTracker(t *testing.T, r *cliRunner, projectIdentifier, tracker, subject string) *issueFixture {
	t.Helper()

	var created struct {
		ID      int    `json:"id"`
		Subject string `json:"subject"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", projectIdentifier,
		"--tracker", tracker,
		"--subject", subject,
		"--description", "Created by redmine-cli e2e tests")
	if created.Subject != subject {
		t.Fatalf("created issue subject = %q, want %q", created.Subject, subject)
	}
	return &issueFixture{ID: created.ID, Subject: created.Subject}
}

// firstTrackerName returns the name of the first tracker visible on the
// server. Tests that just need "any valid tracker" should use this rather
// than hard-coding a name (which differs across default Redmine data sets).
func firstTrackerName(t *testing.T, r *cliRunner) string {
	t.Helper()
	return trackerNames(t, r)[0]
}

func trackerNames(t *testing.T, r *cliRunner) []string {
	t.Helper()
	var trackers []struct {
		Name string `json:"name"`
	}
	r.runJSON(t, &trackers, "trackers", "list")
	if len(trackers) == 0 {
		t.Fatal("trackers list returned no trackers")
	}
	names := make([]string, 0, len(trackers))
	for _, tracker := range trackers {
		names = append(names, tracker.Name)
	}
	return names
}

// firstActivityName fetches /enumerations/time_entry_activities.json via the
// raw api passthrough and returns the first activity. Tests use this so they
// work against any default Redmine data set.
func firstActivityName(t *testing.T, r *cliRunner) string {
	t.Helper()
	var resp struct {
		TimeEntryActivities []struct {
			Name string `json:"name"`
		} `json:"time_entry_activities"`
	}
	r.runJSON(t, &resp, "api", "/enumerations/time_entry_activities.json")
	if len(resp.TimeEntryActivities) == 0 {
		t.Fatal("no time entry activities found on server")
	}
	return resp.TimeEntryActivities[0].Name
}

// issueIDArg renders an issue ID as the string argument expected by the CLI.
func issueIDArg(id int) string { return strconv.Itoa(id) }

// uniqueIdentifier returns a Redmine-valid project identifier seeded from the
// test name plus a nanosecond suffix. Redmine requires identifiers to be
// lowercase letters/digits/dashes and start with a lowercase letter, so we
// always prefix with "e2e-".
func uniqueIdentifier(t *testing.T) string {
	t.Helper()
	id := fmt.Sprintf("e2e-%s-%d", sanitizeForIdentifier(t.Name()), time.Now().UnixNano())
	if len(id) > 100 {
		id = id[:100]
	}
	return id
}

func sanitizeForIdentifier(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}
