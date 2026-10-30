package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type capturedRequest struct {
	method, path, rawQuery, body string
}

func captureServer(t *testing.T, status int, resp string) (*Client, *capturedRequest) {
	t.Helper()
	rec := &capturedRequest{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.method, rec.path, rec.rawQuery, rec.body = r.Method, r.URL.Path, r.URL.RawQuery, string(b)
		if resp != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(ts.Close)
	c := newTestClient(ts)
	c.Categories = &CategoryService{client: c}
	c.Attachments = &AttachmentService{client: c}
	return c, rec
}

func TestCategoryService_Get(t *testing.T) {
	c, rec := captureServer(t, 200, `{"issue_category":{"id":5,"name":"UI","project":{"id":1,"name":"Demo"},"assigned_to":{"id":2,"name":"Alice"}}}`)
	cat, err := c.Categories.Get(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != "GET" || rec.path != "/issue_categories/5.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if cat.ID != 5 || cat.Name != "UI" || cat.Project.Name != "Demo" || cat.AssignedTo == nil || cat.AssignedTo.ID != 2 {
		t.Errorf("category = %+v", cat)
	}
}

func TestCategoryService_Create(t *testing.T) {
	c, rec := captureServer(t, 201, `{"issue_category":{"id":9,"name":"New","project":{"id":1,"name":"Demo"}}}`)
	cat, err := c.Categories.Create(context.Background(), "demo", models.IssueCategoryCreate{Name: "New", AssignedToID: 4})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != "POST" || rec.path != "/projects/demo/issue_categories.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body != `{"issue_category":{"name":"New","assigned_to_id":4}}` {
		t.Errorf("body = %s", rec.body)
	}
	if cat.ID != 9 {
		t.Errorf("category = %+v", cat)
	}
}

func TestCategoryService_Update(t *testing.T) {
	c, rec := captureServer(t, 204, "")
	zero := 0
	name := "Renamed"
	if err := c.Categories.Update(context.Background(), 5, models.IssueCategoryUpdate{Name: &name, AssignedToID: &zero}); err != nil {
		t.Fatal(err)
	}
	if rec.method != "PUT" || rec.path != "/issue_categories/5.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body != `{"issue_category":{"name":"Renamed","assigned_to_id":""}}` {
		t.Errorf("body = %s", rec.body)
	}
}

func TestCategoryService_Delete(t *testing.T) {
	c, rec := captureServer(t, 204, "")
	if err := c.Categories.Delete(context.Background(), 5, 0); err != nil {
		t.Fatal(err)
	}
	if rec.method != "DELETE" || rec.path != "/issue_categories/5.json" || rec.rawQuery != "" {
		t.Errorf("request = %s %s ?%s", rec.method, rec.path, rec.rawQuery)
	}
	if err := c.Categories.Delete(context.Background(), 5, 8); err != nil {
		t.Fatal(err)
	}
	if rec.rawQuery != "reassign_to_id=8" {
		t.Errorf("query = %q, want reassign_to_id=8", rec.rawQuery)
	}
}
