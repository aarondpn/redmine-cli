package ops

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

const newsMe = `{"user":{"id":2,"login":"alice","firstname":"Alice","lastname":"A"}}`

func newsItem(id, author int, title, summary, desc string) string {
	return `{"id":` + strconv.Itoa(id) + `,"project":{"id":1,"name":"Demo"},"author":{"id":` + strconv.Itoa(author) + `,"name":"X"},"title":"` + title + `","summary":"` + summary + `","description":"` + desc + `","created_on":"2024-01-01T00:00:00Z"}`
}

func TestCreateNews_Validation(t *testing.T) {
	client, calls := opsServer(t, nil)
	for name, in := range map[string]CreateNewsInput{
		"no project":     {Title: "T", Description: "D"},
		"no title":       {ProjectID: "demo", Description: "D"},
		"blank title":    {ProjectID: "demo", Title: "  ", Description: "D"},
		"no description": {ProjectID: "demo", Title: "T"},
	} {
		if _, err := CreateNews(context.Background(), client, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestCreateNews_ReadBackMatch(t *testing.T) {
	client, calls := opsServer(t, map[string]string{
		"GET /users/current.json": newsMe,
		// Newest first: a same-content item by someone else, then ours, then an older duplicate.
		"GET /projects/demo/news.json": `{"news":[` +
			newsItem(30, 5, "T", "S", "D") + `,` +
			newsItem(20, 2, "T", "S", "D") + `,` +
			newsItem(10, 2, "T", "S", "D") + `],"total_count":3}`,
	})
	res, err := CreateNews(context.Background(), client, CreateNewsInput{ProjectID: "demo", Title: "T", Summary: "S", Description: "D"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != 20 || len(res.Warnings) != 0 || !strings.Contains(res.Message, "Created news \"T\" (ID: 20)") {
		t.Errorf("result = %+v", res)
	}
	first := (*calls)[0]
	if first.method != "POST" || first.uri != "/projects/demo/news.json" || first.body != `{"news":{"title":"T","summary":"S","description":"D"}}` {
		t.Errorf("first call = %+v", first)
	}
	last := (*calls)[len(*calls)-1]
	if !strings.Contains(last.uri, "limit=25") {
		t.Errorf("read-back uri = %s, want limit=25", last.uri)
	}
}

func TestCreateNews_NoMatchWarns(t *testing.T) {
	client, _ := opsServer(t, map[string]string{
		"GET /users/current.json":      newsMe,
		"GET /projects/demo/news.json": `{"news":[` + newsItem(30, 2, "Other", "S", "D") + `],"total_count":1}`,
	})
	res, err := CreateNews(context.Background(), client, CreateNewsInput{ProjectID: "demo", Title: "T", Summary: "S", Description: "D"})
	if err != nil {
		t.Fatalf("no match must not fail the create: %v", err)
	}
	if res.ID != 0 || len(res.Warnings) != 1 || strings.Contains(res.Message, "Created news 0") {
		t.Errorf("result = %+v", res)
	}
	if !strings.Contains(res.MCPMessage(), "Warning: the new news ID could not be determined") {
		t.Errorf("MCPMessage = %q", res.MCPMessage())
	}
}

func TestCreateNews_ListErrorWarns(t *testing.T) {
	// The default fake answers 204 with no body for the lookups, which cannot be decoded as a list.
	client, _ := opsServer(t, map[string]string{"GET /users/current.json": newsMe, "GET /projects/demo/news.json": `not json`})
	res, err := CreateNews(context.Background(), client, CreateNewsInput{ProjectID: "demo", Title: "T", Description: "D"})
	if err != nil {
		t.Fatalf("read-back failure must not fail the create: %v", err)
	}
	if res.ID != 0 || len(res.Warnings) != 1 {
		t.Errorf("result = %+v", res)
	}
}

func TestUpdateNews(t *testing.T) {
	client, calls := opsServer(t, nil)
	for name, in := range map[string]UpdateNewsInput{
		"no id":        {Title: ptr("x")},
		"nothing":      {ID: 1},
		"empty title":  {ID: 1, Title: ptr(" ")},
		"empty descr.": {ID: 1, Description: ptr("")},
	} {
		if _, err := UpdateNews(context.Background(), client, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("unexpected requests: %v", *calls)
	}
	res, err := UpdateNews(context.Background(), client, UpdateNewsInput{ID: 4, Summary: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if res.Message != "Updated news 4" || c.method != "PUT" || c.uri != "/news/4.json" || c.body != `{"news":{"summary":""}}` {
		t.Errorf("res=%+v call=%+v", res, c)
	}
}

func TestDeleteNews(t *testing.T) {
	client, calls := opsServer(t, nil)
	if _, err := DeleteNews(context.Background(), client, DeleteNewsInput{}); err == nil {
		t.Error("expected error for missing id")
	}
	res, err := DeleteNews(context.Background(), client, DeleteNewsInput{ID: 4})
	if err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if res.Message != "Deleted news 4" || c.method != "DELETE" || c.uri != "/news/4.json" {
		t.Errorf("res=%+v call=%+v", res, c)
	}
}

func TestListNews_DefaultLimit(t *testing.T) {
	client, calls := opsServer(t, map[string]string{"GET /news.json": `{"news":[],"total_count":0}`})
	res, err := ListNews(context.Background(), client, ListNewsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 0 || (*calls)[0].uri != "/news.json?limit=50&offset=0" {
		t.Errorf("res=%+v uri=%s", res, (*calls)[0].uri)
	}
}
