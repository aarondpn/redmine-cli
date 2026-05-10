package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func newQueryResolverClient(t *testing.T, handler http.Handler) (*api.Client, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	c, err := api.NewClient(&config.Config{Server: ts.URL}, debug.New(nil))
	if err != nil {
		t.Fatalf("api.NewClient: %v", err)
	}
	return c, ts.Close
}

func TestResolveQuery_NumericShortCircuits(t *testing.T) {
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected API call to %s", r.URL.Path)
	}))
	defer close()

	q, err := ResolveQuery(context.Background(), client, "42", "")
	if err != nil {
		t.Fatalf("ResolveQuery: %v", err)
	}
	if q.ID != 42 || q.Name != "" {
		t.Fatalf("got %+v, want stub with ID 42 and empty Name", q)
	}
}

func TestResolveQuery_NameMatchesGlobal(t *testing.T) {
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":3,"name":"My open","is_public":false},{"id":4,"name":"All bugs","is_public":true}],"total_count":2}`))
	}))
	defer close()

	q, err := ResolveQuery(context.Background(), client, "all bugs", "")
	if err != nil {
		t.Fatalf("ResolveQuery: %v", err)
	}
	if q.ID != 4 {
		t.Fatalf("got ID %d, want 4", q.ID)
	}
}

func TestResolveQuery_ProjectScopedWinsOverGlobal(t *testing.T) {
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/queries.json":
			_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Sprint","is_public":true},{"id":2,"name":"Sprint","is_public":true,"project_id":7}],"total_count":2}`))
		case "/projects/myproj.json":
			_, _ = w.Write([]byte(`{"project":{"id":7,"name":"My","identifier":"myproj"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer close()

	q, err := ResolveQuery(context.Background(), client, "Sprint", "myproj")
	if err != nil {
		t.Fatalf("ResolveQuery: %v", err)
	}
	if q.ID != 2 {
		t.Fatalf("got ID %d, want 2 (project-scoped)", q.ID)
	}
}

func TestResolveQuery_FallsBackToGlobalWhenProjectHasNoMatch(t *testing.T) {
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/queries.json":
			_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Sprint","is_public":true}],"total_count":1}`))
		case "/projects/myproj.json":
			_, _ = w.Write([]byte(`{"project":{"id":7,"name":"My","identifier":"myproj"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer close()

	q, err := ResolveQuery(context.Background(), client, "Sprint", "myproj")
	if err != nil {
		t.Fatalf("ResolveQuery: %v", err)
	}
	if q.ID != 1 {
		t.Fatalf("got ID %d, want 1 (global fallback)", q.ID)
	}
}

func TestResolveQuery_ExcludesOtherProjectScopedQueries(t *testing.T) {
	// A query named "Sprint" exists only on project 99. With --project=myproj
	// (id 7), neither a global match nor a matching project query exists, so
	// resolution should fail rather than return the unrelated project's
	// query.
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/queries.json":
			_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Sprint","is_public":true,"project_id":99}],"total_count":1}`))
		case "/projects/myproj.json":
			_, _ = w.Write([]byte(`{"project":{"id":7,"name":"My","identifier":"myproj"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer close()

	if _, err := ResolveQuery(context.Background(), client, "Sprint", "myproj"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveQuery_AmbiguousAcrossScopes(t *testing.T) {
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Sprint"},{"id":2,"name":"Sprint","project_id":3}],"total_count":2}`))
	}))
	defer close()

	_, err := ResolveQuery(context.Background(), client, "Sprint", "")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "multiple queries match") {
		t.Fatalf("error = %v, want ambiguity message", err)
	}
}

func TestResolveQuery_AmbiguousMultipleGlobals(t *testing.T) {
	// Two purely-global queries with the same name should also surface as
	// ambiguous. Redmine permits this through the web UI, so the resolver
	// must not silently pick one.
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Sprint","is_public":true},{"id":2,"name":"Sprint","is_public":true}],"total_count":2}`))
	}))
	defer close()

	_, err := ResolveQuery(context.Background(), client, "Sprint", "")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "multiple queries match") {
		t.Fatalf("error = %v, want ambiguity message", err)
	}
}

func TestResolveQuery_NoMatchReturnsSuggestion(t *testing.T) {
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Sprint"}],"total_count":1}`))
	}))
	defer close()

	_, err := ResolveQuery(context.Background(), client, "nope", "")
	if err == nil {
		t.Fatal("expected no-match error")
	}
	if !strings.Contains(err.Error(), "no match") {
		t.Fatalf("error = %v, want no-match message", err)
	}
}

func TestResolveQuery_ForbiddenSurfacesPermissionError(t *testing.T) {
	client, close := newQueryResolverClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer close()

	_, err := ResolveQuery(context.Background(), client, "Sprint", "")
	if err == nil {
		t.Fatal("expected permission error")
	}
	if !IsNameResolutionPermissionError(err) {
		t.Fatalf("error = %v, want NameResolutionPermissionError", err)
	}
}
