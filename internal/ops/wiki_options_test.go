package ops

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

// wikiTreeServer serves a fixed wiki index and records writes:
//
//	Root
//	  Parent
//	    ChildA
//	    ChildB
//	      Grandchild
//	  Other
type wikiTreeServer struct {
	mu       sync.Mutex
	requests []string
	puts     map[string]map[string]any // page -> wiki_page body
	deletes  []string                  // path?query
	parents  map[string]string
	// ignoreParent makes PUTs succeed without changing the parent, like
	// Redmine does without the rename_wiki_pages permission.
	ignoreParent bool
}

func newWikiTreeServer(t *testing.T) (*wikiTreeServer, *api.Client) {
	t.Helper()
	s := &wikiTreeServer{puts: map[string]map[string]any{}, parents: map[string]string{
		"Parent": "Root", "ChildA": "Parent", "ChildB": "Parent", "Grandchild": "ChildB", "Other": "Root", "Root": "",
	}}
	client, closeTS := newTestAPIClient(t, s)
	t.Cleanup(closeTS)
	return s, client
}

func (s *wikiTreeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.Method+" "+r.URL.RequestURI())
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/wiki/index.json"):
		var sb strings.Builder
		sb.WriteString(`{"wiki_pages":[`)
		for i, title := range []string{"Root", "Parent", "ChildA", "ChildB", "Grandchild", "Other"} {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`{"title":"` + title + `","version":1`)
			if p := s.parents[title]; p != "" {
				sb.WriteString(`,"parent":{"title":"` + p + `"}`)
			}
			sb.WriteString("}")
		}
		sb.WriteString(`],"total_count":6}`)
		_, _ = w.Write([]byte(sb.String()))
	case r.Method == http.MethodGet:
		title := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".json")
		if _, err := strconv.Atoi(title); err == nil { // versioned path: /wiki/<page>/<n>.json
			title = strings.TrimSuffix(path[:strings.LastIndex(path, "/")], "/")
			title = title[strings.LastIndex(title, "/")+1:]
		}
		body := `{"wiki_page":{"title":"` + title + `","text":"body of ` + title + `","version":3`
		if p := s.parents[title]; p != "" {
			body += `,"parent":{"title":"` + p + `"}`
		}
		_, _ = w.Write([]byte(body + "}}"))
	case r.Method == http.MethodPut:
		b, _ := io.ReadAll(r.Body)
		var parsed map[string]map[string]any
		_ = json.Unmarshal(b, &parsed)
		title := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".json")
		s.puts[title] = parsed["wiki_page"]
		if p, ok := parsed["wiki_page"]["parent_title"].(string); ok && !s.ignoreParent {
			s.parents[title] = p
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete:
		s.deletes = append(s.deletes, r.URL.RequestURI())
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func TestDeleteWikiPage_TodoValues(t *testing.T) {
	for _, tt := range []struct {
		name, todo, want string
	}{
		{"default sends no todo", "", "/projects/p/wiki/Parent.json"},
		{"nullify", "nullify", "/projects/p/wiki/Parent.json?todo=nullify"},
		{"destroy", "destroy", "/projects/p/wiki/Parent.json?todo=destroy"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, client := newWikiTreeServer(t)
			res, err := DeleteWikiPage(t.Context(), client, DeleteWikiPageInput{ProjectID: "p", Page: "Parent", Todo: tt.todo})
			if err != nil {
				t.Fatal(err)
			}
			if len(s.deletes) != 1 || s.deletes[0] != tt.want {
				t.Errorf("deletes = %v, want [%s]", s.deletes, tt.want)
			}
			if tt.todo == "destroy" && !strings.Contains(res.Message, "descendants") {
				t.Errorf("message = %q, want it to mention descendants", res.Message)
			}
		})
	}
}

func TestDeleteWikiPage_Validation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input DeleteWikiPageInput
		want  string
	}{
		{"unknown todo", DeleteWikiPageInput{Todo: "burn"}, "todo must be nullify or destroy"},
		{"reassign is not offered", DeleteWikiPageInput{Todo: "reassign"}, "todo must be nullify or destroy"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, client := newWikiTreeServer(t)
			tt.input.ProjectID, tt.input.Page = "p", "Parent"
			_, err := DeleteWikiPage(t.Context(), client, tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want substring %q", err, tt.want)
			}
			if len(s.deletes) != 0 {
				t.Errorf("nothing may be sent on a rejected request, got deletes=%v", s.deletes)
			}
		})
	}
}

func TestUpdateWikiPage_Options(t *testing.T) {
	yes, no := true, false
	title := "Renamed"

	s, client := newWikiTreeServer(t)
	// The fake keeps the old title on read-back; only the body matters here.
	_, _ = UpdateWikiPage(t.Context(), client, UpdateWikiPageInput{
		ProjectID: "p", Page: "Other", Title: &title,
		IsStartPage: true, RedirectExistingLinks: &no, DeletedAttachmentIDs: []int{7},
	})
	wp := s.puts["Other"]
	if wp["is_start_page"] != true || wp["redirect_existing_links"] != "0" {
		t.Errorf("body = %v", wp)
	}
	if ids, _ := wp["deleted_attachment_ids"].([]any); len(ids) != 1 || ids[0] != float64(7) {
		t.Errorf("deleted_attachment_ids = %v", wp["deleted_attachment_ids"])
	}

	s, client = newWikiTreeServer(t)
	_, _ = UpdateWikiPage(t.Context(), client, UpdateWikiPageInput{
		ProjectID: "p", Page: "Other", Title: &title, RedirectExistingLinks: &yes,
	})
	if s.puts["Other"]["redirect_existing_links"] != "1" {
		t.Errorf("body = %v, want redirect_existing_links \"1\"", s.puts["Other"])
	}
}

func TestUpdateWikiPage_OptionGuards(t *testing.T) {
	yes := true
	empty := ""
	for _, tt := range []struct {
		name  string
		input UpdateWikiPageInput
		want  string
	}{
		{"redirect without title", UpdateWikiPageInput{RedirectExistingLinks: &yes}, "requires title"},
		{"redirect with empty title", UpdateWikiPageInput{RedirectExistingLinks: &yes, Title: &empty}, "requires title"},
		{"bad attachment id", UpdateWikiPageInput{DeletedAttachmentIDs: []int{3, 0}}, "positive attachment IDs"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.ProjectID, tt.input.Page = "p", "Page"
			_, err := UpdateWikiPage(t.Context(), nil, tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestGetWikiPage_Version(t *testing.T) {
	s, client := newWikiTreeServer(t)
	v := 2
	page, err := GetWikiPage(t.Context(), client, GetWikiPageInput{ProjectID: "p", Page: "Other", Version: &v})
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Other" {
		t.Errorf("title = %q", page.Title)
	}
	if got := s.requests[0]; got != "GET /projects/p/wiki/Other/2.json" {
		t.Errorf("request = %q, want the versioned path", got)
	}

	zero := 0
	if _, err := GetWikiPage(t.Context(), client, GetWikiPageInput{ProjectID: "p", Page: "Other", Version: &zero}); err == nil {
		t.Error("version 0 must be rejected")
	}
	if _, err := GetWikiPage(t.Context(), client, GetWikiPageInput{ProjectID: "p", Page: "Other", Version: &v, Includes: []string{"attachments"}}); err != nil {
		t.Fatal(err)
	}
	if got := s.requests[len(s.requests)-1]; got != "GET /projects/p/wiki/Other/2.json?include=attachments" {
		t.Errorf("request = %q, want the versioned path with include", got)
	}
}
