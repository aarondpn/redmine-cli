package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestFileService_List(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "files": [
                {
                    "id": 7,
                    "filename": "release.tar.gz",
                    "filesize": 12345,
                    "content_type": "application/gzip",
                    "description": "v1 release",
                    "content_url": "https://example/attachments/download/7/release.tar.gz",
                    "author": {"id": 1, "name": "Alice"},
                    "created_on": "2026-04-01T10:00:00Z",
                    "version": {"id": 4, "name": "1.0"},
                    "digest": "abc123",
                    "downloads": 2
                }
            ],
            "total_count": 1
        }`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Files = &FileService{client: c}

	files, total, err := c.Files.List(context.Background(), "proj a", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	want := "/projects/proj%20a/files.json"
	if gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	got := files[0]
	if got.ID != 7 || got.Filename != "release.tar.gz" || got.Filesize != 12345 {
		t.Errorf("file = %+v, unexpected core fields", got)
	}
	if got.Version == nil || got.Version.Name != "1.0" {
		t.Errorf("version = %+v, want name=1.0", got.Version)
	}
	if got.Author.Name != "Alice" {
		t.Errorf("author = %+v, want Alice", got.Author)
	}
}

func TestFileService_Create_Body(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   map[string]interface{}
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Files = &FileService{client: c}

	err := c.Files.Create(context.Background(), "proj", models.ProjectFileCreate{
		Token:       "tok-1",
		Filename:    "build.zip",
		VersionID:   42,
		Description: "release artifact",
		ContentType: "application/zip",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/projects/proj/files.json" {
		t.Errorf("path = %q, want /projects/proj/files.json", gotPath)
	}
	file, ok := gotBody["file"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing file key")
	}
	if file["token"] != "tok-1" {
		t.Errorf("token = %v, want tok-1", file["token"])
	}
	if file["filename"] != "build.zip" {
		t.Errorf("filename = %v, want build.zip", file["filename"])
	}
	if file["version_id"] != float64(42) {
		t.Errorf("version_id = %v, want 42", file["version_id"])
	}
	if file["description"] != "release artifact" {
		t.Errorf("description = %v, want release artifact", file["description"])
	}
	if file["content_type"] != "application/zip" {
		t.Errorf("content_type = %v, want application/zip", file["content_type"])
	}
}

func TestFileService_Create_OmitsZeroValueFields(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Files = &FileService{client: c}

	if err := c.Files.Create(context.Background(), "proj", models.ProjectFileCreate{Token: "tok"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	file, ok := gotBody["file"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing file key")
	}
	for _, key := range []string{"filename", "version_id", "description", "content_type"} {
		if _, present := file[key]; present {
			t.Errorf("%s should be omitted when empty, got %v", key, file[key])
		}
	}
}
