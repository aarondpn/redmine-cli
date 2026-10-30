package ops

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

// TestUpdateWikiPage_RejectsZeroVersion guards the ops-layer boundary: the
// CLI already validates --expect-version, but MCP clients (or any other
// programmatic caller) reach this function directly and could send a
// pointer-to-zero. The JSON omitempty rule on *int would serialize that as
// `"version":0`, which Redmine rejects — we'd rather catch it locally with
// a clear message than round-trip a useless request.
func TestUpdateWikiPage_RejectsZeroVersion(t *testing.T) {
	zero := 0
	negative := -3

	for name, version := range map[string]*int{
		"zero":     &zero,
		"negative": &negative,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := UpdateWikiPage(t.Context(), nil, UpdateWikiPageInput{
				ProjectID: "proj",
				Page:      "MyPage",
				Version:   version,
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), ">= 1") {
				t.Errorf("error = %q, want it to mention the >= 1 constraint", err.Error())
			}
		})
	}
}

// TestUpdateWikiPage_SectionGuards covers the ops-layer section validation.
// These guards matter most for MCP/programmatic callers that bypass the CLI
// flag checks: a non-positive section, a section edit without replacement
// text (which would otherwise resend the whole page into one section), or a
// section_hash with no section to anchor it. A nil client is safe because
// validation runs before any request is made.
func TestUpdateWikiPage_SectionGuards(t *testing.T) {
	str := func(s string) *string { return &s }
	intp := func(i int) *int { return &i }
	text := str("section body")

	cases := []struct {
		name    string
		input   UpdateWikiPageInput
		wantSub string
	}{
		{
			name:    "zero section",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Text: text, Section: intp(0)},
			wantSub: "section must be >= 1",
		},
		{
			name:    "negative section",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Text: text, Section: intp(-2)},
			wantSub: "section must be >= 1",
		},
		{
			name:    "section without text",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Section: intp(2)},
			wantSub: "section update requires text",
		},
		{
			name:    "section_hash without section",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Text: text, SectionHash: str("abc123")},
			wantSub: "section_hash requires section",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := UpdateWikiPage(t.Context(), nil, tc.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// wikiParentServer is a fake Redmine for parent changes. Its PUT answers like
// the real one (201 with the page for a new page, 204 otherwise) regardless
// of whether parent_title was applied; the read-back GET returns pageGet.
type wikiParentServer struct {
	mu      sync.Mutex
	putBody map[string]any
	gets    []string
	// putStatus/putResp control the PUT response. putStatus 0 means 204.
	putStatus int
	putResp   string
	pageGet   string // JSON body for the read-back GET; empty means 500
	// pages overrides pageGet per request path: a JSON body, or "404" for
	// a missing page.
	pages map[string]string
}

func (s *wikiParentServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &s.putBody)
		if s.putStatus == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.putStatus)
		_, _ = w.Write([]byte(s.putResp))
	case http.MethodGet:
		s.gets = append(s.gets, r.URL.Path)
		if body, ok := s.pages[r.URL.Path]; ok {
			if body == "404" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":["Not found"]}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
			return
		}
		if s.pageGet == "" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.pageGet))
	default:
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}
}

func (s *wikiParentServer) sentParent(t *testing.T) (any, bool) {
	t.Helper()
	return s.sentField(t, "parent_title")
}

func (s *wikiParentServer) sentField(t *testing.T, key string) (any, bool) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	wp, ok := s.putBody["wiki_page"].(map[string]any)
	if !ok {
		t.Fatalf("PUT body missing wiki_page: %#v", s.putBody)
	}
	v, present := wp[key]
	return v, present
}

func strPtr(s string) *string { return &s }

func TestUpdateWikiPage_ParentApplied(t *testing.T) {
	// Redmine titleizes and matches case-insensitively, so "user guide"
	// resolving to "User_Guide" counts as applied.
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","parent":{"title":"User_Guide"},"version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("user guide"),
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
	if got, _ := srv.sentParent(t); got != "user guide" {
		t.Errorf("sent parent_title = %v, want the caller's input unchanged", got)
	}
	if len(srv.gets) != 1 || srv.gets[0] != "/projects/proj/wiki/Install.json" {
		t.Errorf("read-back GETs = %v, want one GET of the page", srv.gets)
	}
}

func TestUpdateWikiPage_ParentCleared(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	if _, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr(""),
	}); err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	got, present := srv.sentParent(t)
	if !present || got != "" {
		t.Errorf("parent_title = %v (present %v), want an explicit empty string", got, present)
	}
}

func TestUpdateWikiPage_ParentNotApplied(t *testing.T) {
	// Without rename_wiki_pages Redmine drops parent_title and still answers
	// 204; the read-back shows the old parent.
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","parent":{"title":"Old_Parent"},"version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("New Parent"),
	})
	var notApplied *WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *WikiParentNotAppliedError", err)
	}
	if notApplied.Page != "Install" || notApplied.Requested != "New Parent" || notApplied.Current != "Old_Parent" {
		t.Errorf("error = %+v, want page Install, requested New Parent, current Old_Parent", notApplied)
	}
	for _, want := range []string{`"New Parent"`, `"Old_Parent"`, "rename_wiki_pages"} {
		if !strings.Contains(notApplied.Summary(), want) {
			t.Errorf("Summary() = %q, want it to mention %s", notApplied.Summary(), want)
		}
	}
}

func TestUpdateWikiPage_ClearParentNotApplied(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","parent":{"title":"Guide"},"version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr(""),
	})
	var notApplied *WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *WikiParentNotAppliedError", err)
	}
	if !strings.Contains(notApplied.Summary(), "none (wiki root)") {
		t.Errorf("Summary() = %q, want it to name the wiki root as the requested parent", notApplied.Summary())
	}
}

func TestUpdateWikiPage_NoParentSkipsReadBack(t *testing.T) {
	srv := &wikiParentServer{}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	if _, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"),
	}); err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	if _, present := srv.sentParent(t); present {
		t.Error("parent_title sent although no parent was requested")
	}
	if len(srv.gets) != 0 {
		t.Errorf("read-back GETs = %v, want none without a parent change", srv.gets)
	}
}

// TestUpdateWikiPage_ParentReadBackUsesNewTitle: after a rename the old
// title redirects to the HTML page, which the API cannot follow, so the page
// is read back under its new title.
func TestUpdateWikiPage_ParentReadBackUsesNewTitle(t *testing.T) {
	srv := &wikiParentServer{pages: map[string]string{
		"/projects/proj/wiki/Install.json": `<html>redirected</html>`,
		"/projects/proj/wiki/Renamed.json": `{"wiki_page":{"title":"Renamed","parent":{"title":"Guide"},"version":2}}`,
	}}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	if _, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), Title: strPtr("Renamed"), ParentTitle: strPtr("Guide"),
	}); err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	want := []string{"/projects/proj/wiki/Install.json", "/projects/proj/wiki/Renamed.json"}
	if strings.Join(srv.gets, " ") != strings.Join(want, " ") {
		t.Errorf("read-back GETs = %v, want %v", srv.gets, want)
	}
}

// TestUpdateWikiPage_RenameOntoTakenTitleDropped: without rename_wiki_pages
// Redmine drops a rename onto a title another page already has, so the read
// back must not compare that unrelated page's parent.
func TestUpdateWikiPage_RenameOntoTakenTitleDropped(t *testing.T) {
	srv := &wikiParentServer{pages: map[string]string{
		"/projects/proj/wiki/Install.json": `{"wiki_page":{"title":"Install","version":2}}`,
		"/projects/proj/wiki/Taken.json":   `{"wiki_page":{"title":"Taken","parent":{"title":"Guide"},"version":5}}`,
	}}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), Title: strPtr("Taken"), ParentTitle: strPtr("Guide"),
	})
	var notApplied *WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *WikiParentNotAppliedError", err)
	}
	if notApplied.Page != "Install" || notApplied.DroppedTitle != "Taken" {
		t.Errorf("error = %+v, want page Install with dropped title Taken", notApplied)
	}
}

func TestUpdateWikiPage_ParentReadBackFailureIsWarning(t *testing.T) {
	srv := &wikiParentServer{}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("Guide"),
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage: %v (a failed read-back must not turn an applied update into an error)", err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "could not verify") {
		t.Fatalf("Warnings = %v, want one verification warning", res.Warnings)
	}
	if !strings.Contains(res.MCPMessage(), "Warning: could not verify") {
		t.Errorf("MCPMessage() = %q, want the warning appended", res.MCPMessage())
	}
}

func TestCreateWikiPage_ParentApplied(t *testing.T) {
	srv := &wikiParentServer{
		putStatus: http.StatusCreated,
		putResp:   `{"wiki_page":{"title":"Install","parent":{"title":"Guide"},"version":1}}`,
	}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	page, err := CreateWikiPage(context.Background(), client, CreateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: "body", ParentTitle: "Guide",
	})
	if err != nil {
		t.Fatalf("CreateWikiPage: %v", err)
	}
	if page.Parent == nil || page.Parent.Title != "Guide" {
		t.Errorf("page.Parent = %+v, want Guide", page.Parent)
	}
	if got, _ := srv.sentParent(t); got != "Guide" {
		t.Errorf("sent parent_title = %v, want Guide", got)
	}
	if len(srv.gets) != 0 {
		t.Errorf("GETs = %v, want none: the create response carries the parent", srv.gets)
	}
}

// TestCreateWikiPage_OverwriteParentNotApplied covers create over an existing
// page: Redmine treats it as an update, answers 204 and may drop the parent.
func TestCreateWikiPage_OverwriteParentNotApplied(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := CreateWikiPage(context.Background(), client, CreateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: "body", ParentTitle: "Guide",
	})
	var notApplied *WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *WikiParentNotAppliedError", err)
	}
	if notApplied.Current != "" {
		t.Errorf("Current = %q, want empty (page is at the wiki root)", notApplied.Current)
	}
}

// TestWikiParent_InvalidParentGetsExplained covers the bare 422 Redmine sends
// for an unknown or circular parent: it renders the content's (empty) errors
// instead of the page's, so the caller would otherwise get no reason at all.
func TestWikiParent_InvalidParentGetsExplained(t *testing.T) {
	for name, call := range map[string]func(*api.Client) error{
		"create": func(c *api.Client) error {
			_, err := CreateWikiPage(context.Background(), c, CreateWikiPageInput{
				ProjectID: "proj", Page: "Install", Text: "body", ParentTitle: "Nope",
			})
			return err
		},
		"update": func(c *api.Client) error {
			_, err := UpdateWikiPage(context.Background(), c, UpdateWikiPageInput{
				ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("Nope"),
			})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			srv := &wikiParentServer{putStatus: http.StatusUnprocessableEntity, putResp: `{"errors":[]}`}
			client, closeTS := newTestAPIClient(t, srv)
			defer closeTS()

			err := call(client)
			var apiErr *api.APIError
			if !errors.As(err, &apiErr) || !apiErr.IsValidationError() {
				t.Fatalf("err = %v, want a 422 *api.APIError", err)
			}
			if len(apiErr.Errors) != 1 || !strings.Contains(apiErr.Errors[0], `Parent page "Nope"`) {
				t.Errorf("Errors = %v, want one message naming the rejected parent", apiErr.Errors)
			}
		})
	}
}

// TestWikiParent_ServerMessagesKept makes sure the hint never replaces real
// validation messages from Redmine.
func TestWikiParent_ServerMessagesKept(t *testing.T) {
	srv := &wikiParentServer{putStatus: http.StatusUnprocessableEntity, putResp: `{"errors":["Text cannot be blank"]}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr(""), ParentTitle: strPtr("Guide"),
	})
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *api.APIError", err)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0] != "Text cannot be blank" {
		t.Errorf("Errors = %v, want the server's message unchanged", apiErr.Errors)
	}
}

func TestWikiTitleize(t *testing.T) {
	for in, want := range map[string]string{
		"User Guide":       "User_Guide",
		"a  b\tc":          "a_b_c",
		"Q1, 2026: Notes.": "Q1_2026_Notes",
		"a/b?c;d|e":        "abcde",
		"":                 "",
	} {
		if got := wikiTitleize(in); got != want {
			t.Errorf("wikiTitleize(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestUpdateWikiPage_WithoutTextResendsCurrent covers the natural MCP call
// shape for a move ({page, parent_title}): Redmine rejects an update without
// text, so the current text is fetched and resent.
func TestUpdateWikiPage_WithoutTextResendsCurrent(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","text":"current body","parent":{"title":"Guide"},"version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	if _, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", ParentTitle: strPtr("Guide"),
	}); err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	if got, _ := srv.sentField(t, "text"); got != "current body" {
		t.Errorf("sent text = %v, want the current body resent", got)
	}
}

// TestUpdateWikiPage_ParentAndRenameDropped: Redmine drops title and
// parent_title together without rename_wiki_pages, so the new title does not
// exist and the page must be read back under its old one.
func TestUpdateWikiPage_ParentAndRenameDropped(t *testing.T) {
	srv := &wikiParentServer{pages: map[string]string{
		"/projects/proj/wiki/Renamed.json": "404",
		"/projects/proj/wiki/Install.json": `{"wiki_page":{"title":"Install","version":2}}`,
		"/projects/proj/wiki/Guide.json":   `{"wiki_page":{"title":"Guide","version":1}}`,
	}}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), Title: strPtr("Renamed"), ParentTitle: strPtr("Guide"),
	})
	var notApplied *WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *WikiParentNotAppliedError", err)
	}
	// The dropped rename already proves the parent was dropped, so the
	// requested parent is not looked up.
	for _, path := range srv.gets {
		if path == "/projects/proj/wiki/Guide.json" {
			t.Errorf("looked up the requested parent although the dropped rename settles it: %v", srv.gets)
		}
	}
	if notApplied.Page != "Install" || notApplied.DroppedTitle != "Renamed" {
		t.Errorf("error = %+v, want page Install with dropped title Renamed", notApplied)
	}
	if !strings.Contains(strings.Join(notApplied.Details(), " "), `rename to "Renamed" was not applied`) {
		t.Errorf("Details() = %q, want the dropped rename named", notApplied.Details())
	}
}

func TestUpdateWikiPage_RenameDroppedParentUnchangedWarns(t *testing.T) {
	srv := &wikiParentServer{pages: map[string]string{
		"/projects/proj/wiki/Renamed.json": "404",
		"/projects/proj/wiki/Install.json": `{"wiki_page":{"title":"Install","parent":{"title":"Guide"},"version":2}}`,
	}}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), Title: strPtr("Renamed"), ParentTitle: strPtr("Guide"),
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], `not renamed to "Renamed"`) {
		t.Errorf("Warnings = %v, want a note about the dropped rename", res.Warnings)
	}
}

// TestUpdateWikiPage_ParentViaRedirectIsInconclusive: Redmine follows rename
// redirects for parent_title, so a parent named by an old title lands on the
// renamed page. The API cannot resolve the redirect (Redmine answers with an
// HTML redirect), so the mismatch must be a warning, not an error that makes
// callers retry a change that did land.
func TestUpdateWikiPage_ParentViaRedirectIsInconclusive(t *testing.T) {
	srv := &wikiParentServer{pages: map[string]string{
		"/projects/proj/wiki/Install.json": `{"wiki_page":{"title":"Install","parent":{"title":"User_Guide"},"version":2}}`,
		"/projects/proj/wiki/Guide.json":   `<html>redirected</html>`,
	}}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("Guide"),
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage: %v (an unresolvable parent title must not be reported as a dropped change)", err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "rename redirects") {
		t.Errorf("Warnings = %v, want one note explaining the possible redirect", res.Warnings)
	}
}

// TestUpdateWikiPage_MissingParentDropped: a parent that does not exist
// would have been rejected with 422 had Redmine applied the field, so a
// silent success means it was dropped.
func TestUpdateWikiPage_MissingParentDropped(t *testing.T) {
	srv := &wikiParentServer{pages: map[string]string{
		"/projects/proj/wiki/Install.json": `{"wiki_page":{"title":"Install","version":2}}`,
		"/projects/proj/wiki/Nope.json":    "404",
	}}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("Nope"),
	})
	var notApplied *WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *WikiParentNotAppliedError", err)
	}
}

// TestUpdateWikiPage_BlankParentMeansRoot mirrors Redmine's blank? check:
// a whitespace-only parent_title moves the page to the root.
func TestUpdateWikiPage_BlankParentMeansRoot(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("  "),
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
}

func TestWikiParentNotAppliedError_Details(t *testing.T) {
	bare := &WikiParentNotAppliedError{Page: "P", Requested: "A"}
	if len(bare.Details()) != 0 || bare.Error() != bare.Summary() {
		t.Errorf("without other changes: Details() = %q, Error() = %q", bare.Details(), bare.Error())
	}
	full := &WikiParentNotAppliedError{Page: "P", Requested: "A", DroppedTitle: "Q", OtherChanges: true}
	if len(full.Details()) != 2 {
		t.Fatalf("Details() = %q, want the dropped rename and the other-changes note", full.Details())
	}
	if !strings.Contains(full.Details()[1], "may already have been applied") {
		t.Errorf("Details()[1] = %q", full.Details()[1])
	}
}

func TestAnnotateWikiParentError_WithTitle(t *testing.T) {
	err := annotateWikiParentError(api.NewAPIError(http.StatusUnprocessableEntity, "/x", nil), "Guide", "Taken")
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || len(apiErr.Errors) != 1 {
		t.Fatalf("err = %v, want one annotated message", err)
	}
	for _, want := range []string{`"Guide"`, `"Taken"`} {
		if !strings.Contains(apiErr.Errors[0], want) {
			t.Errorf("message %q does not mention %s", apiErr.Errors[0], want)
		}
	}
	blank := annotateWikiParentError(api.NewAPIError(http.StatusUnprocessableEntity, "/x", nil), " ", "")
	if errors.As(blank, &apiErr) && len(apiErr.Errors) != 0 {
		t.Errorf("blank parent annotated: %v", apiErr.Errors)
	}
}

func TestUpdateWikiPage_WithoutTextPinsVersion(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","text":"current body","parent":{"title":"Guide"},"version":4}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	if _, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", ParentTitle: strPtr("Guide"),
	}); err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	if got, _ := srv.sentField(t, "version"); got != float64(4) {
		t.Errorf("sent version = %v, want the fetched version 4 so a concurrent edit gets 409", got)
	}
}

// TestUpdateWikiPage_WithoutTextStaleVersionConflicts: Redmine skips the
// version check when the text does not change, so a stale version must be
// rejected locally before anything is sent.
func TestUpdateWikiPage_WithoutTextStaleVersionConflicts(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","text":"current body","version":7}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	stale := 3
	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", ParentTitle: strPtr("Guide"), Version: &stale,
	})
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || !apiErr.IsConflict() {
		t.Fatalf("err = %v, want a 409 *api.APIError", err)
	}
	if srv.putBody != nil {
		t.Errorf("PUT sent despite the stale version: %#v", srv.putBody)
	}
}

func TestUpdateWikiPage_ParentOnlyHasNoOtherChangesNote(t *testing.T) {
	srv := &wikiParentServer{pageGet: `{"wiki_page":{"title":"Install","text":"body","version":2}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", ParentTitle: strPtr("Guide"),
	})
	var notApplied *WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *WikiParentNotAppliedError", err)
	}
	if len(notApplied.Details()) != 0 {
		t.Errorf("Details() = %q, want none: only the parent was requested", notApplied.Details())
	}
}

// TestUpdateWikiPage_ParentResolvesToStoredParent covers a lookup that
// disagrees with the local title comparison: if Redmine resolves want to the
// stored parent, the change landed and must not be reported as dropped.
func TestUpdateWikiPage_ParentResolvesToStoredParent(t *testing.T) {
	srv := &wikiParentServer{pages: map[string]string{
		"/projects/proj/wiki/Install.json": `{"wiki_page":{"title":"Install","parent":{"title":"User_Guide"},"version":2}}`,
		"/projects/proj/wiki/Guide.json":   `{"wiki_page":{"title":"User_Guide","version":1}}`,
	}}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := UpdateWikiPage(context.Background(), client, UpdateWikiPageInput{
		ProjectID: "proj", Page: "Install", Text: strPtr("body"), ParentTitle: strPtr("Guide"),
	})
	if err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
}
