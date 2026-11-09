package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestWikiService_DeleteWithTodo(t *testing.T) {
	for _, tt := range []struct {
		todo      string
		wantQuery string
		wantErr   bool
	}{
		{"", "", false},
		{WikiTodoNullify, "todo=nullify", false},
		{WikiTodoDestroy, "todo=destroy", false},
		{"reassign", "", true},
		{"bogus", "", true},
	} {
		t.Run(tt.todo, func(t *testing.T) {
			var gotMethod, gotPath, gotQuery string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
				w.WriteHeader(http.StatusNoContent)
			}))
			defer ts.Close()
			c := newTestClient(ts)
			c.Wikis = &WikiService{client: c}

			err := c.Wikis.DeleteWithTodo(context.Background(), "proj", "Parent", tt.todo)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if gotMethod != "" {
					t.Errorf("request sent despite error: %s", gotMethod)
				}
				return
			}
			if gotMethod != http.MethodDelete || gotPath != "/projects/proj/wiki/Parent.json" || gotQuery != tt.wantQuery {
				t.Errorf("got %s %s?%s, want DELETE /projects/proj/wiki/Parent.json?%s", gotMethod, gotPath, gotQuery, tt.wantQuery)
			}
		})
	}
}

func TestWikiService_Update_SendsOptions(t *testing.T) {
	var body map[string]map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text, title, no := "t", "New", "0"
	err := c.Wikis.Update(context.Background(), "proj", "Old", models.WikiPageUpdate{
		Text: &text, Title: &title,
		IsStartPage:           true,
		RedirectExistingLinks: &no,
		DeletedAttachmentIDs:  []int{4, 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	wp := body["wiki_page"]
	if wp["is_start_page"] != true {
		t.Errorf("is_start_page = %v, want true", wp["is_start_page"])
	}
	// Redmine only treats the string "0" as off, so it must not be a JSON bool.
	if wp["redirect_existing_links"] != "0" {
		t.Errorf("redirect_existing_links = %#v, want \"0\"", wp["redirect_existing_links"])
	}
	ids, _ := wp["deleted_attachment_ids"].([]any)
	if len(ids) != 2 || ids[0] != float64(4) || ids[1] != float64(9) {
		t.Errorf("deleted_attachment_ids = %v, want [4 9]", wp["deleted_attachment_ids"])
	}
}

func TestWikiService_Update_OmitsUnsetOptions(t *testing.T) {
	var raw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		raw = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	c := newTestClient(ts)
	c.Wikis = &WikiService{client: c}

	text := "t"
	if err := c.Wikis.Update(context.Background(), "proj", "P", models.WikiPageUpdate{Text: &text}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"is_start_page", "redirect_existing_links", "deleted_attachment_ids"} {
		if strings.Contains(raw, k) {
			t.Errorf("body %s contains %s", raw, k)
		}
	}
}
