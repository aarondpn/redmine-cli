//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// pngFixture returns a small payload that begins with the real PNG signature
// and contains NUL and high bytes, so a byte-identical round-trip proves
// binary integrity (no text decoding / newline mangling).
func pngFixture(uniq string) []byte {
	return []byte("\x89PNG\r\n\x1a\n\x00\x01\x02\x03\xff\xfe redmine-cli e2e " + uniq + "\n")
}

// createIssueWithAttachment creates an issue carrying a single attachment and
// returns the issue ID, the on-disk filename, and the exact bytes uploaded.
func createIssueWithAttachment(t *testing.T, r *cliRunner, projectIdentifier string) (issueID int, filename string, payload []byte) {
	t.Helper()
	uniq := strconv.FormatInt(time.Now().UnixNano(), 36)
	filename = "logo-" + uniq + ".png"
	payload = pngFixture(uniq)
	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write attach file: %v", err)
	}

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", projectIdentifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E attachment "+uniq,
		"--attach", path)
	if created.ID == 0 {
		t.Fatal("issues create returned no ID")
	}
	return created.ID, filename, payload
}

type attachmentEntry struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Filesize    int64  `json:"filesize"`
	ContentType string `json:"content_type"`
}

// findIssueAttachmentID discovers the attachment ID via the new
// `issues get --attachments` flag, asserting the attachment surfaces in the
// typed JSON output.
func findIssueAttachmentID(t *testing.T, r *cliRunner, issueID int, filename string) attachmentEntry {
	t.Helper()
	var resp struct {
		Attachments []attachmentEntry `json:"attachments"`
	}
	r.runJSON(t, &resp, "issues", "get", issueIDArg(issueID), "--attachments")
	for _, att := range resp.Attachments {
		if att.Filename == filename {
			return att
		}
	}
	t.Fatalf("attachment %q not surfaced by `issues get --attachments`: %+v", filename, resp.Attachments)
	return attachmentEntry{}
}

// TestAttachments_Lifecycle exercises the full discover -> metadata -> download
// flow and asserts binary integrity of the downloaded file.
func TestAttachments_Lifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	issueID, filename, payload := createIssueWithAttachment(t, r, proj.Identifier)

	att := findIssueAttachmentID(t, r, issueID, filename)
	if att.ID == 0 {
		t.Fatal("discovered attachment has zero ID")
	}
	if att.Filesize != int64(len(payload)) {
		t.Errorf("filesize = %d, want %d", att.Filesize, len(payload))
	}

	// Metadata via `attachments get`.
	var meta attachmentEntry
	r.runJSON(t, &meta, "attachments", "get", strconv.Itoa(att.ID))
	if meta.Filename != filename || meta.Filesize != int64(len(payload)) {
		t.Errorf("attachments get = %+v, want filename=%q size=%d", meta, filename, len(payload))
	}

	// Download into a directory using the real filename.
	dir := t.TempDir()
	var dl struct {
		ID       int    `json:"id"`
		Filename string `json:"filename"`
		Path     string `json:"path"`
		Bytes    int64  `json:"bytes"`
	}
	r.runJSON(t, &dl, "attachments", "download", strconv.Itoa(att.ID), "--dir", dir)
	if dl.Path != filepath.Join(dir, filename) {
		t.Errorf("download path = %q, want %q", dl.Path, filepath.Join(dir, filename))
	}
	if dl.Bytes != int64(len(payload)) {
		t.Errorf("downloaded bytes = %d, want %d", dl.Bytes, len(payload))
	}

	got, err := os.ReadFile(dl.Path)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("downloaded file not byte-identical to the uploaded PNG\nwant %v\ngot  %v", payload, got)
	}
}

// TestAttachments_DownloadToStdout verifies `--path -` streams raw bytes to
// stdout without corruption, even though the runner injects --output json.
func TestAttachments_DownloadToStdout(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	issueID, filename, payload := createIssueWithAttachment(t, r, proj.Identifier)
	att := findIssueAttachmentID(t, r, issueID, filename)

	stdout, _, err := r.runRaw("attachments", "download", strconv.Itoa(att.ID), "--path", "-")
	if err != nil {
		t.Fatalf("download to stdout failed: %v", err)
	}
	if !bytes.Equal(stdout, payload) {
		t.Fatalf("stdout not byte-identical to the uploaded PNG\nwant %v\ngot  %v", payload, stdout)
	}
}

// TestAttachments_IssueDownloadAll covers `issues get --download-attachments`.
func TestAttachments_IssueDownloadAll(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	issueID, filename, payload := createIssueWithAttachment(t, r, proj.Identifier)

	dir := filepath.Join(t.TempDir(), "issue-attachments")
	// The issue JSON still lands on stdout; the download is a stderr-reported
	// side effect, so we only need a successful exit here.
	r.run(t, "issues", "get", issueIDArg(issueID), "--download-attachments", dir)

	got, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("expected downloaded attachment in %s: %v", dir, err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("issue-downloaded attachment not byte-identical to source")
	}
}

// TestAttachments_GetMissing asserts a clear error for an unknown attachment.
func TestAttachments_GetMissing(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout, _ := r.runExpectError(t, "attachments", "get", "999999999")
	requireErrorEnvelopeMessage(t, stdout)
}
