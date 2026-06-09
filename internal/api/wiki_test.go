package api

import (
	"context"
	"encoding/json"
	"errors"
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
		return
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

func TestWikiService_Update_WithVersion_SendsVersionInBody(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text := "rewritten body"
	version := 7
	err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
		Text:    &text,
		Version: &version,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wp, ok := gotBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing wiki_page key")
	}
	// JSON numbers decode as float64.
	got, ok := wp["version"].(float64)
	if !ok {
		t.Fatalf("version not present or wrong type in body: %#v", wp["version"])
	}
	if int(got) != version {
		t.Errorf("body version = %v, want %d", got, version)
	}
}

func TestWikiService_Update_WithoutVersion_OmitsField(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text := "rewritten body"
	err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
		Text: &text,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wp, ok := gotBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing wiki_page key")
	}
	if _, present := wp["version"]; present {
		t.Errorf("version should be omitted when nil, got %#v", wp["version"])
	}
}

func TestWikiService_Update_Conflict_ReturnsAPIErrorIsConflict(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"errors":["Page has been updated by someone else"]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text := "rewritten body"
	version := 1
	err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
		Text:    &text,
		Version: &version,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsConflict() {
		t.Errorf("IsConflict = false, want true (status %d)", apiErr.StatusCode)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0] != "Page has been updated by someone else" {
		t.Errorf("apiErr.Errors = %v, want server-provided message", apiErr.Errors)
	}
}

func TestWikiService_Update_WithSection_SendsSectionInBody(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text := "section content"
	section := 5
	err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
		Text:    &text,
		Section: &section,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wp, ok := gotBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing wiki_page key")
	}
	got, ok := wp["section"].(float64)
	if !ok {
		t.Fatalf("section not present or wrong type in body: %#v", wp["section"])
	}
	if int(got) != section {
		t.Errorf("body section = %v, want %d", got, section)
	}
}

func TestWikiService_Update_WithSectionHash_SendsSectionHashInBody(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text := "section content"
	section := 3
	sectionHash := "abc123def456"
	err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
		Text:        &text,
		Section:     &section,
		SectionHash: &sectionHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wp, ok := gotBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing wiki_page key")
	}
	if wp["section_hash"] != sectionHash {
		t.Errorf("body section_hash = %v, want %q", wp["section_hash"], sectionHash)
	}
}

func TestWikiService_Update_WithoutSection_OmitsSectionField(t *testing.T) {
	var gotBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text := "rewritten body"
	err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
		Text: &text,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wp, ok := gotBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatal("body missing wiki_page key")
	}
	if _, present := wp["section"]; present {
		t.Errorf("section should be omitted when nil, got %#v", wp["section"])
	}
	if _, present := wp["section_hash"]; present {
		t.Errorf("section_hash should be omitted when nil, got %#v", wp["section_hash"])
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
