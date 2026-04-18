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

func TestWikiService_Get_URLEscaping(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use EscapedPath to see the %-encoded form; r.URL.Path is decoded.
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"wiki_page":{"title":"My Page/Draft","version":1}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	_, err := c.Wikis.Get(context.Background(), "foo", "My Page/Draft", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "My Page/Draft" must be a single escaped path segment, not split into "My Page" and "Draft".
	want := "/projects/foo/wiki/My%20Page%2FDraft.json"
	if gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestWikiService_Get_WithIncludes(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"wiki_page":{"title":"Test","version":1}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	_, err := c.Wikis.Get(context.Background(), "proj", "Test", []string{"attachments"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "include=attachments" {
		t.Errorf("query = %q, want include=attachments", gotQuery)
	}
}

func TestWikiService_GetVersion(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"wiki_page":{"title":"Test","version":3}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	_, err := c.Wikis.GetVersion(context.Background(), "proj", "Test", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/projects/proj/wiki/Test/3.json"
	if gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestWikiService_Create_Body(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"wiki_page":{"title":"NewPage","version":1}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	page, err := c.Wikis.Create(context.Background(), "proj", "NewPage", models.WikiPageCreate{
		Text:     "Hello world",
		Comments: "created",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page == nil {
		t.Fatal("expected non-nil page")
	}
	if page.Title != "NewPage" {
		t.Errorf("page.Title = %q, want NewPage", page.Title)
	}
	if page.Version != 1 {
		t.Errorf("page.Version = %d, want 1", page.Version)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	wantPath := "/projects/proj/wiki/NewPage.json"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	wp, ok := gotBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing wiki_page key")
	}
	if wp["text"] != "Hello world" {
		t.Errorf("text = %v, want Hello world", wp["text"])
	}
	if wp["comments"] != "created" {
		t.Errorf("comments = %v, want created", wp["comments"])
	}
}

func TestWikiService_Update_TextFallback(t *testing.T) {
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
	c.Wikis = &WikiService{client: c}

	existingText := "the original page content"
	err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
		Text: &existingText,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	wantPath := "/projects/proj/wiki/MyPage.json"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	wp, ok := gotBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing wiki_page key")
	}
	if wp["text"] != existingText {
		t.Errorf("text = %v, want %q", wp["text"], existingText)
	}
}

func TestWikiService_Delete(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	err := c.Wikis.Delete(context.Background(), "proj", "OldPage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	wantPath := "/projects/proj/wiki/OldPage.json"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
}
