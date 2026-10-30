package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWikiService_Create_ParentTitle(t *testing.T) {
	for name, tc := range map[string]struct {
		parent      string
		wantPresent bool
	}{
		"set":     {parent: "User Guide", wantPresent: true},
		"omitted": {parent: "", wantPresent: false},
	} {
		t.Run(name, func(t *testing.T) {
			var gotBody map[string]interface{}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &gotBody)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"wiki_page":{"title":"Child","parent":{"title":"User_Guide"},"version":1}}`))
			}))
			defer ts.Close()

			c := newTestClient(ts)
			c.Wikis = &WikiService{client: c}

			page, err := c.Wikis.Create(context.Background(), "proj", "Child", models.WikiPageCreate{
				Text:        "body",
				ParentTitle: tc.parent,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page.Parent == nil || page.Parent.Title != "User_Guide" {
				t.Errorf("page.Parent = %+v, want User_Guide", page.Parent)
			}
			wp, ok := gotBody["wiki_page"].(map[string]interface{})
			if !ok {
				t.Fatal("body missing wiki_page key")
			}
			got, present := wp["parent_title"]
			if present != tc.wantPresent {
				t.Fatalf("parent_title present = %v, want %v (body %#v)", present, tc.wantPresent, wp)
			}
			if tc.wantPresent && got != tc.parent {
				t.Errorf("parent_title = %v, want %q", got, tc.parent)
			}
		})
	}
}

// TestWikiService_Create_EmptyBodyReadBack covers the fallback after a 204
// overwrite with --title: the original name is read first (Redmine drops
// the rename without rename_wiki_pages, and the new title may belong to an
// unrelated page), and the new title only when the original is gone.
func TestWikiService_Create_EmptyBodyReadBack(t *testing.T) {
	for name, tc := range map[string]struct {
		pages     map[string]string
		wantTitle string
		wantGets  []string
	}{
		"rename dropped": {
			pages: map[string]string{
				"/projects/proj/wiki/Old.json": `{"wiki_page":{"title":"Old","version":2}}`,
				"/projects/proj/wiki/New.json": `{"wiki_page":{"title":"New","version":9}}`,
			},
			wantTitle: "Old",
			wantGets:  []string{"/projects/proj/wiki/Old.json"},
		},
		"rename applied": {
			pages: map[string]string{
				"/projects/proj/wiki/New.json": `{"wiki_page":{"title":"New","version":2}}`,
			},
			wantTitle: "New",
			wantGets:  []string{"/projects/proj/wiki/Old.json", "/projects/proj/wiki/New.json"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var gets []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				gets = append(gets, r.URL.Path)
				body, ok := tc.pages[r.URL.Path]
				w.Header().Set("Content-Type", "application/json")
				if !ok {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"errors":["Not found"]}`))
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer ts.Close()

			c := newTestClient(ts)
			c.Wikis = &WikiService{client: c}

			page, err := c.Wikis.Create(context.Background(), "proj", "Old", models.WikiPageCreate{Text: "body", Title: "New"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page.Title != tc.wantTitle {
				t.Errorf("page.Title = %q, want %q", page.Title, tc.wantTitle)
			}
			if strings.Join(gets, " ") != strings.Join(tc.wantGets, " ") {
				t.Errorf("GETs = %v, want %v", gets, tc.wantGets)
			}
		})
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

	// Redmine reads the section selector from the top level of the params,
	// not from inside wiki_page, so it must be sent there.
	got, ok := gotBody["section"].(float64)
	if !ok {
		t.Fatalf("section not present or wrong type at top level of body: %#v", gotBody["section"])
	}
	if int(got) != section {
		t.Errorf("body section = %v, want %d", got, section)
	}
	if wp, ok := gotBody["wiki_page"].(map[string]interface{}); ok {
		if _, present := wp["section"]; present {
			t.Errorf("section must not be nested under wiki_page, got %#v", wp["section"])
		}
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

	if gotBody["section_hash"] != sectionHash {
		t.Errorf("top-level body section_hash = %v, want %q", gotBody["section_hash"], sectionHash)
	}
	if wp, ok := gotBody["wiki_page"].(map[string]interface{}); ok {
		if _, present := wp["section_hash"]; present {
			t.Errorf("section_hash must not be nested under wiki_page, got %#v", wp["section_hash"])
		}
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

	if _, ok := gotBody["wiki_page"].(map[string]interface{}); !ok {
		t.Fatal("body missing wiki_page key")
	}
	if _, present := gotBody["section"]; present {
		t.Errorf("section should be omitted when nil, got %#v", gotBody["section"])
	}
	if _, present := gotBody["section_hash"]; present {
		t.Errorf("section_hash should be omitted when nil, got %#v", gotBody["section_hash"])
	}
}

// TestWikiService_Update_ParentTitle covers the three states of the pointer:
// nil leaves the parent alone (field omitted), a title moves the page, and
// the empty string must still be sent so Redmine moves the page to the root.
func TestWikiService_Update_ParentTitle(t *testing.T) {
	str := func(s string) *string { return &s }
	for name, tc := range map[string]struct {
		parent      *string
		wantPresent bool
	}{
		"nil omits":     {parent: nil, wantPresent: false},
		"title is sent": {parent: str("User Guide"), wantPresent: true},
		"empty is sent": {parent: str(""), wantPresent: true},
	} {
		t.Run(name, func(t *testing.T) {
			var gotBody map[string]interface{}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &gotBody)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer ts.Close()

			c := newTestClient(ts)
			c.Wikis = &WikiService{client: c}

			text := "body"
			if err := c.Wikis.Update(context.Background(), "proj", "MyPage", models.WikiPageUpdate{
				Text:        &text,
				ParentTitle: tc.parent,
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			wp, ok := gotBody["wiki_page"].(map[string]interface{})
			if !ok {
				t.Fatal("body missing wiki_page key")
			}
			got, present := wp["parent_title"]
			if present != tc.wantPresent {
				t.Fatalf("parent_title present = %v, want %v (body %#v)", present, tc.wantPresent, wp)
			}
			if tc.wantPresent && got != *tc.parent {
				t.Errorf("parent_title = %v, want %q", got, *tc.parent)
			}
		})
	}
}

func TestWikiService_List_DecodesHierarchy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"wiki_pages":[` +
			`{"title":"Guide","version":3,"created_on":"2026-01-01T00:00:00Z","updated_on":"2026-01-02T00:00:00Z"},` +
			`{"title":"Install","parent":{"title":"Guide"},"version":1,"project":{"id":7,"name":"Proj"},` +
			`"created_on":"2026-01-03T00:00:00Z","updated_on":"2026-01-04T00:00:00Z"}]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	pages, _, err := c.Wikis.List(context.Background(), "proj", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("len(pages) = %d, want 2", len(pages))
	}
	if pages[0].Parent != nil {
		t.Errorf("top-level page parent = %+v, want nil", pages[0].Parent)
	}
	if pages[0].Version != 3 || pages[0].CreatedOn != "2026-01-01T00:00:00Z" {
		t.Errorf("pages[0] = %+v, want version 3 and created_on set", pages[0])
	}
	if pages[1].Parent == nil || pages[1].Parent.Title != "Guide" {
		t.Errorf("pages[1].Parent = %+v, want Guide", pages[1].Parent)
	}
	if pages[1].Project == nil || pages[1].Project.ID != 7 {
		t.Errorf("pages[1].Project = %+v, want id 7", pages[1].Project)
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
