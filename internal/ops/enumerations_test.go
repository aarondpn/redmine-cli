package ops

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

func enumTestClient(t *testing.T, h http.HandlerFunc) *api.Client {
	t.Helper()
	client, closeTS := newTestAPIClient(t, h)
	t.Cleanup(closeTS)
	return client
}

func TestListPriorities_Count(t *testing.T) {
	client := enumTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/enumerations/issue_priorities.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"issue_priorities":[{"id":1,"name":"Low"},{"id":2,"name":"Normal","is_default":true}]}`))
	})
	res, err := ListPriorities(context.Background(), client, struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 2 || res.Priorities[1].Name != "Normal" || !res.Priorities[1].IsDefault {
		t.Errorf("result = %+v", res)
	}
}

func TestListDocumentCategories_Count(t *testing.T) {
	client := enumTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/enumerations/document_categories.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"document_categories":[{"id":1,"name":"User documentation"}]}`))
	})
	res, err := ListDocumentCategories(context.Background(), client, struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 || res.DocumentCategories[0].ID != 1 {
		t.Errorf("result = %+v", res)
	}
}

func TestListTimeEntryActivities_Global(t *testing.T) {
	client := enumTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/enumerations/time_entry_activities.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"time_entry_activities":[{"id":9,"name":"Design","active":true}]}`))
	})
	res, err := ListTimeEntryActivities(context.Background(), client, ListTimeEntryActivitiesInput{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 || res.Activities[0].ID != 9 {
		t.Errorf("result = %+v", res)
	}
}

func TestListTimeEntryActivities_Project(t *testing.T) {
	client := enumTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo.json" || r.URL.Query().Get("include") != "time_entry_activities" {
			t.Errorf("request = %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo","time_entry_activities":[{"id":50,"name":"Design"},{"id":10,"name":"Development"}]}}`))
	})
	res, err := ListTimeEntryActivities(context.Background(), client, ListTimeEntryActivitiesInput{ProjectID: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 2 || res.Activities[0].ID != 50 {
		t.Errorf("result = %+v", res)
	}
}

func TestListTimeEntryActivities_ProjectKeyAbsent(t *testing.T) {
	client := enumTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
	})
	_, err := ListTimeEntryActivities(context.Background(), client, ListTimeEntryActivitiesInput{ProjectID: "demo"})
	if err == nil || !strings.Contains(err.Error(), "view_time_entries") {
		t.Fatalf("err = %v, want view_time_entries hint", err)
	}
}
