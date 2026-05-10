package wiki

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// TestUpdate_ExpectVersion_SendsVersion verifies that --expect-version is
// propagated as the wiki_page.version field on the PUT body.
func TestUpdate_ExpectVersion_SendsVersion(t *testing.T) {
	var (
		mu       sync.Mutex
		putBody  map[string]interface{}
		putCount int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// The project resolver fetches /projects/<id>.json before any
			// wiki call. Reply with a minimal project payload so the
			// resolver returns cleanly.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			putCount++
			_ = json.Unmarshal(b, &putBody)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request method %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{
		"MyPage",
		"--project", "proj",
		"--text", "rewritten body",
		"--expect-version", "7",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if putCount != 1 {
		t.Fatalf("expected exactly one PUT, got %d", putCount)
	}
	wp, ok := putBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatalf("body missing wiki_page key: %#v", putBody)
	}
	got, ok := wp["version"].(float64)
	if !ok {
		t.Fatalf("version not present in PUT body: %#v", wp["version"])
	}
	if int(got) != 7 {
		t.Errorf("body version = %v, want 7", got)
	}
}

// TestUpdate_EnsureCurrent_FetchesAndSendsCurrentVersion verifies the
// convenience mode: the command first GETs the current page, then PUTs back
// using the freshly fetched version.
func TestUpdate_EnsureCurrent_FetchesAndSendsCurrentVersion(t *testing.T) {
	var (
		mu       sync.Mutex
		wikiGets int
		putBody  map[string]interface{}
		putCount int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/wiki/") {
				mu.Lock()
				wikiGets++
				mu.Unlock()
				_, _ = w.Write([]byte(`{"wiki_page":{"title":"MyPage","text":"existing","version":4}}`))
				return
			}
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			putCount++
			_ = json.Unmarshal(b, &putBody)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{
		"MyPage",
		"--project", "proj",
		"--text", "rewritten body",
		"--ensure-current",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wikiGets < 1 {
		t.Errorf("expected at least one wiki GET to fetch current version, got %d", wikiGets)
	}
	if putCount != 1 {
		t.Fatalf("expected exactly one PUT, got %d", putCount)
	}
	wp, ok := putBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatalf("body missing wiki_page key: %#v", putBody)
	}
	got, ok := wp["version"].(float64)
	if !ok {
		t.Fatalf("version not present in PUT body: %#v", wp["version"])
	}
	if int(got) != 4 {
		t.Errorf("body version = %v, want 4 (the value returned by GET)", got)
	}
}

// TestUpdate_NoVersionFlag_OmitsVersion guards the existing default behavior:
// when neither --expect-version nor --ensure-current is set, the PUT body
// must not carry a version field, so server-side optimistic locking is
// opt-in and existing scripts keep working.
func TestUpdate_NoVersionFlag_OmitsVersion(t *testing.T) {
	var (
		mu      sync.Mutex
		putBody map[string]interface{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/wiki/") {
				_, _ = w.Write([]byte(`{"wiki_page":{"title":"MyPage","text":"existing","version":2}}`))
				return
			}
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			_ = json.Unmarshal(b, &putBody)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{
		"MyPage",
		"--project", "proj",
		"--text", "rewritten body",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wp, ok := putBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatalf("body missing wiki_page key: %#v", putBody)
	}
	if _, present := wp["version"]; present {
		t.Errorf("version should be absent without --expect-version/--ensure-current, got %v", wp["version"])
	}
}

// TestUpdate_MutuallyExclusiveFlags rejects passing both flags at once.
func TestUpdate_MutuallyExclusiveFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be hit, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{
		"MyPage",
		"--project", "proj",
		"--text", "x",
		"--expect-version", "3",
		"--ensure-current",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want it to mention mutual exclusivity", err.Error())
	}
}

// TestUpdate_Conflict_ReportsStaleVersion verifies that a 409 from the server
// surfaces as an actionable error mentioning that the page has been
// modified.
func TestUpdate_Conflict_ReportsStaleVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
		case http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"errors":["Page has been updated by someone else"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{
		"MyPage",
		"--project", "proj",
		"--text", "rewritten body",
		"--expect-version", "1",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}

	// The CLI hands the error to cmdutil.FormatError before printing it.
	// That path is what the user actually sees, so assert against it: the
	// generic "Conflict" prefix is present, the wiki-specific context line
	// the command injects survives, and the server-provided detail is
	// preserved alongside it.
	formatted := cmdutil.FormatError(err)
	for _, want := range []string{
		"Conflict",
		`wiki page "MyPage" has been modified since version 1`,
		"Page has been updated by someone else",
	} {
		if !strings.Contains(formatted, want) {
			t.Errorf("FormatError output missing %q\nfull:\n%s", want, formatted)
		}
	}

	// And the JSON envelope path must classify the error as code=conflict
	// so scripts can branch on it.
	env := cmdutil.BuildErrorEnvelope(err)
	if env.Error.Code != output.ErrCodeConflict {
		t.Errorf("envelope code = %q, want %q", env.Error.Code, output.ErrCodeConflict)
	}
}

// TestUpdate_ExpectVersionWithTitle_StillSendsVersion guards the ordering
// inside RunE: the version assignment happens before --title is applied, so
// renaming a page with optimistic concurrency must still send the asserted
// version. A regression that re-ordered or short-circuited the switch would
// silently drop the version.
func TestUpdate_ExpectVersionWithTitle_StillSendsVersion(t *testing.T) {
	var (
		mu      sync.Mutex
		putBody map[string]interface{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			_ = json.Unmarshal(b, &putBody)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{
		"MyPage",
		"--project", "proj",
		"--text", "rewritten",
		"--title", "Renamed Page",
		"--expect-version", "5",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wp, ok := putBody["wiki_page"].(map[string]interface{})
	if !ok {
		t.Fatalf("body missing wiki_page key: %#v", putBody)
	}
	v, ok := wp["version"].(float64)
	if !ok {
		t.Fatalf("version not in PUT body: %#v", wp["version"])
	}
	if int(v) != 5 {
		t.Errorf("version = %v, want 5", v)
	}
	if got, _ := wp["title"].(string); got != "Renamed Page" {
		t.Errorf("title = %q, want Renamed Page", got)
	}
}

// TestUpdate_ExpectVersionMustBePositive guards against passing zero or
// negative values to --expect-version, which would silently match Redmine's
// notion of "no version asserted" because of JSON omitempty rules.
func TestUpdate_ExpectVersionMustBePositive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be hit, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{
		"MyPage",
		"--project", "proj",
		"--text", "x",
		"--expect-version", "0",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), ">= 1") {
		t.Errorf("error = %q, want a >= 1 message", err.Error())
	}
}
