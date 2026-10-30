package api

import (
	"context"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func newsClient(t *testing.T, status int, resp string) (*Client, *capturedRequest) {
	t.Helper()
	c, rec := captureServer(t, status, resp)
	c.News = &NewsService{client: c}
	return c, rec
}

func TestNewsService_List_Global(t *testing.T) {
	c, rec := newsClient(t, 200, `{"news":[{"id":3,"project":{"id":1,"name":"Demo"},"author":{"id":2,"name":"Alice"},"title":"T","summary":null,"description":"D","created_on":"2024-01-01T00:00:00Z"}],"total_count":7,"offset":0,"limit":25}`)
	items, total, err := c.News.List(context.Background(), "", 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != "GET" || rec.path != "/news.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if total != 7 || len(items) != 1 || items[0].ID != 3 || items[0].Summary != "" || items[0].Author.Name != "Alice" {
		t.Errorf("items = %+v total = %d", items, total)
	}
}

func TestNewsService_List_ProjectOffset(t *testing.T) {
	c, rec := newsClient(t, 200, `{"news":[],"total_count":0}`)
	if _, _, err := c.News.List(context.Background(), "demo", 10, 20); err != nil {
		t.Fatal(err)
	}
	if rec.path != "/projects/demo/news.json" {
		t.Errorf("path = %s", rec.path)
	}
	if rec.rawQuery != "limit=10&offset=20" && rec.rawQuery != "offset=20&limit=10" {
		t.Errorf("query = %s", rec.rawQuery)
	}
}

func TestNewsService_Get_Includes(t *testing.T) {
	c, rec := newsClient(t, 200, `{"news":{"id":3,"title":"T","comments":[{"id":1,"author":{"id":2,"name":"Alice"},"content":"hi"}],"attachments":[{"id":8,"filename":"a.txt"}]}}`)
	n, err := c.News.Get(context.Background(), 3, []string{"attachments", "comments"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.path != "/news/3.json" || rec.rawQuery != "include=attachments%2Ccomments" {
		t.Errorf("request = %s ?%s", rec.path, rec.rawQuery)
	}
	if len(n.Comments) != 1 || n.Comments[0].Content != "hi" || len(n.Attachments) != 1 {
		t.Errorf("news = %+v", n)
	}
}

func TestNewsService_Create(t *testing.T) {
	c, rec := newsClient(t, 204, "")
	err := c.News.Create(context.Background(), "demo", models.NewsCreate{
		Title: "T", Summary: "S", Description: "D",
		Uploads: []models.Upload{{Token: "tok", Filename: "a.txt"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != "POST" || rec.path != "/projects/demo/news.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	want := `{"news":{"title":"T","summary":"S","description":"D","uploads":[{"token":"tok","filename":"a.txt"}]}}`
	if rec.body != want {
		t.Errorf("body = %s", rec.body)
	}
}

func TestNewsService_Update_OmitsUnsetFields(t *testing.T) {
	c, rec := newsClient(t, 204, "")
	empty := ""
	if err := c.News.Update(context.Background(), 3, models.NewsUpdate{Summary: &empty}); err != nil {
		t.Fatal(err)
	}
	if rec.method != "PUT" || rec.path != "/news/3.json" || rec.body != `{"news":{"summary":""}}` {
		t.Errorf("request = %s %s %s", rec.method, rec.path, rec.body)
	}
}

func TestNewsService_Delete(t *testing.T) {
	c, rec := newsClient(t, 204, "")
	if err := c.News.Delete(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if rec.method != "DELETE" || rec.path != "/news/3.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestNewsService_Get_NotFound(t *testing.T) {
	c, _ := newsClient(t, 404, "")
	if _, err := c.News.Get(context.Background(), 99, nil); err == nil {
		t.Fatal("expected error")
	}
}
