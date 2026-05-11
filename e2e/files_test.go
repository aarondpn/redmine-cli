//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestFiles_Lifecycle exercises the project files command group end-to-end:
// upload a file to a fresh project, then list it and verify the metadata
// round-trips. The Redmine /projects/:id/files.json endpoint returns 204 on
// upload and reveals the resource only via the list endpoint, so the lifecycle
// must combine the two to be meaningful.
func TestFiles_Lifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	tmp := t.TempDir()
	uniq := strconv.FormatInt(time.Now().UnixNano(), 36)
	filename := "release-" + uniq + ".txt"
	path := filepath.Join(tmp, filename)
	payload := []byte("file uploaded by redmine-cli e2e " + uniq + "\n")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var uploaded actionEnvelope
	r.runJSON(t, &uploaded, "files", "upload", path,
		"--project", proj.Identifier,
		"--description", "e2e fixture")
	if !uploaded.Ok || uploaded.Action != "uploaded" || uploaded.Resource != "project_file" {
		t.Fatalf("unexpected upload envelope: %+v", uploaded)
	}

	type fileEntry struct {
		ID          int    `json:"id"`
		Filename    string `json:"filename"`
		Filesize    int64  `json:"filesize"`
		ContentType string `json:"content_type"`
		Description string `json:"description"`
	}
	var listed []fileEntry
	r.runJSON(t, &listed, "files", "list", "--project", proj.Identifier)

	var found *fileEntry
	for i, f := range listed {
		if f.Filename == filename {
			found = &listed[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("uploaded file %q not in list: %+v", filename, listed)
	}
	if found.Filesize != int64(len(payload)) {
		t.Errorf("filesize = %d, want %d", found.Filesize, len(payload))
	}
	if found.Description != "e2e fixture" {
		t.Errorf("description = %q, want %q", found.Description, "e2e fixture")
	}
}

// TestFiles_ListEmptyProject covers the empty-state path so a regression in
// the no-results envelope or pagination handling does not slip past CI.
func TestFiles_ListEmptyProject(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	var listed []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &listed, "files", "list", "--project", proj.Identifier)
	if len(listed) != 0 {
		t.Errorf("expected empty list for fresh project, got %+v", listed)
	}
}

// TestFiles_UploadMissingFile checks that a clear error is surfaced when the
// path does not exist, instead of a partial upload or an opaque API error.
func TestFiles_UploadMissingFile(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	stdout, _ := r.runExpectError(t, "files", "upload",
		filepath.Join(t.TempDir(), "does-not-exist.bin"),
		"--project", proj.Identifier)
	requireErrorEnvelopeMessage(t, stdout)
}
