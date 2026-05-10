package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrackerServiceGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trackers.json" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug","default_status":{"id":2,"name":"New"},"enabled_standard_fields":["description"]}]}`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	client.Trackers = &TrackerService{client: client}
	tracker, err := client.Trackers.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tracker.Name != "Bug" {
		t.Fatalf("tracker name = %q, want Bug", tracker.Name)
	}
	if tracker.DefaultStatus == nil || tracker.DefaultStatus.Name != "New" {
		t.Fatalf("default status = %+v, want New", tracker.DefaultStatus)
	}
	if len(tracker.EnabledStandardFields) != 1 || tracker.EnabledStandardFields[0] != "description" {
		t.Fatalf("enabled standard fields = %v", tracker.EnabledStandardFields)
	}
}

func TestTrackerServiceGetNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trackers.json" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug"}]}`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	client.Trackers = &TrackerService{client: client}
	_, err := client.Trackers.Get(context.Background(), 99)
	if err == nil {
		t.Fatal("Get returned nil error, want not found")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Get error type = %T, want *APIError", err)
	}
	if !apiErr.IsNotFound() {
		t.Fatalf("Get error status = %d, want 404", apiErr.StatusCode)
	}
}
