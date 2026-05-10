package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomFieldServiceGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom_fields.json" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"custom_fields":[
			{"id":1,"name":"Severity","customized_type":"issue","field_format":"list","is_required":true,"possible_values":[{"value":"Low","label":"Low"},{"value":"High","label":"High"}],"trackers":[{"id":1,"name":"Bug"}]},
			{"id":2,"name":"Department","customized_type":"user","field_format":"string"}
		]}`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	client.CustomFields = &CustomFieldService{client: client}

	field, err := client.CustomFields.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if field.Name != "Severity" {
		t.Fatalf("name = %q, want Severity", field.Name)
	}
	if field.CustomizedType != "issue" || field.FieldFormat != "list" {
		t.Fatalf("type/format = %q/%q", field.CustomizedType, field.FieldFormat)
	}
	if !field.IsRequired {
		t.Fatal("is_required should be true")
	}
	if len(field.PossibleValues) != 2 || field.PossibleValues[0].Value != "Low" {
		t.Fatalf("possible_values = %+v", field.PossibleValues)
	}
	if len(field.Trackers) != 1 || field.Trackers[0].Name != "Bug" {
		t.Fatalf("trackers = %+v", field.Trackers)
	}
}

func TestCustomFieldServiceGetNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"custom_fields":[{"id":1,"name":"Severity"}]}`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	client.CustomFields = &CustomFieldService{client: client}
	_, err := client.CustomFields.Get(context.Background(), 99)
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

func TestCustomFieldServiceListPropagatesForbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":["You are not authorized to access this page."]}`))
	}))
	defer ts.Close()

	client := newTestClient(ts)
	client.CustomFields = &CustomFieldService{client: client}
	_, err := client.CustomFields.List(context.Background())
	if err == nil {
		t.Fatal("List returned nil error, want forbidden")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("List error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("List error status = %d, want 403", apiErr.StatusCode)
	}
}
