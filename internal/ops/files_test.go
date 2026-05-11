package ops

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// TestUploadProjectFile_RequiresToken guards the ops-layer boundary: the CLI
// always calls /uploads.json first, but MCP or other programmatic callers
// reach UploadProjectFile directly. Without this check an empty token would
// round-trip to Redmine and surface as an opaque 422.
func TestUploadProjectFile_RequiresToken(t *testing.T) {
	_, err := UploadProjectFile(context.Background(), nil, UploadProjectFileInput{
		ProjectID: "proj",
		Filename:  "x.txt",
	})
	if err == nil {
		t.Fatal("expected error when token is empty")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error = %q, want it to mention the token requirement", err.Error())
	}
}

func TestUploadProjectFile_PassesThroughToAPI(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))

	result, err := UploadProjectFile(context.Background(), client, UploadProjectFileInput{
		ProjectID:   "proj",
		Token:       "tok",
		Filename:    "build.zip",
		VersionID:   3,
		Description: "release",
		ContentType: "application/zip",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message == "" {
		t.Error("expected non-empty success message")
	}
	file, ok := gotBody["file"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing file key")
	}
	if file["token"] != "tok" {
		t.Errorf("token = %v, want tok", file["token"])
	}
	if file["filename"] != "build.zip" {
		t.Errorf("filename = %v, want build.zip", file["filename"])
	}
	if file["version_id"] != float64(3) {
		t.Errorf("version_id = %v, want 3", file["version_id"])
	}
}

func TestListProjectFiles_DecodesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "files": [
                {"id": 1, "filename": "a.txt", "filesize": 10, "author": {"id": 5, "name": "Bob"}, "created_on": "2026-04-02T10:00:00Z"},
                {"id": 2, "filename": "b.zip", "filesize": 200, "author": {"id": 5, "name": "Bob"}, "created_on": "2026-04-03T10:00:00Z", "version": {"id": 9, "name": "1.1"}}
            ],
            "total_count": 2
        }`))
	}))
	defer ts.Close()

	client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))

	got, err := ListProjectFiles(context.Background(), client, ListProjectFilesInput{ProjectID: "proj"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TotalCount != 2 || got.Count != 2 {
		t.Errorf("counts = (count=%d, total=%d), want both 2", got.Count, got.TotalCount)
	}
	if got.Files[1].Version == nil || got.Files[1].Version.Name != "1.1" {
		t.Errorf("version = %+v, want name=1.1", got.Files[1].Version)
	}
}
