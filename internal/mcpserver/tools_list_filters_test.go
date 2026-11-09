package mcpserver

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

// filterInputTypes lists the list tool inputs whose extra_params must
// advertise string-or-array values.
var filterInputTypes = map[string]any{
	"ListIssuesInput":      inputSchemaFor[ops.ListIssuesInput](),
	"ListTimeEntriesInput": inputSchemaFor[ops.ListTimeEntriesInput](),
	"ListProjectsInput":    inputSchemaFor[ops.ListProjectsInput](),
	"ListUsersInput":       inputSchemaFor[ops.ListUsersInput](),
}

func TestInputSchemaFor_ExtraParams(t *testing.T) {
	for name, raw := range filterInputTypes {
		t.Run(name, func(t *testing.T) {
			schema, ok := raw.(*jsonschema.Schema)
			if !ok || schema == nil {
				t.Fatalf("schema = %T, want *jsonschema.Schema", raw)
			}
			prop := schema.Properties["extra_params"]
			if prop == nil {
				t.Fatal("no extra_params property")
			}
			if prop.Type != "object" || prop.Description == "" {
				t.Errorf("extra_params = %+v", prop)
			}
			if prop.PropertyNames != nil {
				t.Errorf("extra_params keys must not be restricted: %+v", prop.PropertyNames)
			}
			ap := prop.AdditionalProperties
			if ap == nil || len(ap.AnyOf) != 2 || ap.AnyOf[0].Type != "string" ||
				ap.AnyOf[1].Type != "array" || ap.AnyOf[1].Items == nil || ap.AnyOf[1].Items.Type != "string" {
				t.Fatalf("additionalProperties = %+v, want string or array of strings", ap)
			}
		})
	}
}

// listQueryServer answers every list endpoint with an empty collection and
// records each request by path.
func listQueryServer(t *testing.T) (http.Handler, func(path string) url.Values) {
	t.Helper()
	var (
		mu   sync.Mutex
		seen = map[string]url.Values{}
	)
	keys := map[string]string{
		"/issues.json":       "issues",
		"/time_entries.json": "time_entries",
		"/projects.json":     "projects",
		"/users.json":        "users",
		"/groups.json":       "groups",
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := keys[r.URL.Path]
		if !ok {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		mu.Lock()
		seen[r.URL.Path] = r.URL.Query()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"` + key + `":[],"total_count":0}`))
	})
	return h, func(path string) url.Values {
		mu.Lock()
		defer mu.Unlock()
		q, ok := seen[path]
		if !ok {
			t.Fatalf("no request to %s", path)
		}
		return q
	}
}

func wantMCPQuery(t *testing.T, q url.Values, key string, want ...string) {
	t.Helper()
	if !slices.Equal(q[key], want) {
		t.Errorf("%s = %q, want %q (query %s)", key, q[key], want, q.Encode())
	}
}

func TestListTools_FiltersReachTheQueryString(t *testing.T) {
	h, query := listQueryServer(t)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	call := func(name string, args map[string]any) {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError {
			t.Fatalf("%s returned error: %s", name, asText(t, res.Content[0]))
		}
	}

	call("list_issues", map[string]any{
		"status_id":    "open",
		"extra_params": map[string]any{"status_id": "*", "f[]": []string{"status_id", "subject"}, "cf_5": "Critical"},
	})
	q := query("/issues.json")
	wantMCPQuery(t, q, "status_id", "*")
	wantMCPQuery(t, q, "f[]", "status_id", "subject")
	wantMCPQuery(t, q, "cf_5", "Critical")

	call("list_time_entries", map[string]any{
		"sort":         "spent_on:desc",
		"query_id":     7,
		"extra_params": map[string]any{"spent_on": "><2026-01-01|2026-01-31", "v[hours][]": []string{"1", "2"}},
	})
	q = query("/time_entries.json")
	wantMCPQuery(t, q, "sort", "spent_on:desc")
	wantMCPQuery(t, q, "query_id", "7")
	wantMCPQuery(t, q, "spent_on", "><2026-01-01|2026-01-31")
	wantMCPQuery(t, q, "v[hours][]", "1", "2")

	call("list_projects", map[string]any{"status": "closed", "sort": "name", "extra_params": map[string]any{"name": "~web"}})
	q = query("/projects.json")
	wantMCPQuery(t, q, "status", "5")
	wantMCPQuery(t, q, "sort", "name")
	wantMCPQuery(t, q, "name", "~web")

	call("list_users", map[string]any{"status": "all", "sort": "login", "extra_params": map[string]any{"op[status]": "*"}})
	q = query("/users.json")
	wantMCPQuery(t, q, "status", "")
	wantMCPQuery(t, q, "sort", "login")
	wantMCPQuery(t, q, "op[status]", "*")

	call("list_groups", map[string]any{"builtin": true})
	wantMCPQuery(t, query("/groups.json"), "builtin", "1")
}

func TestListTools_RejectInvalidFilters(t *testing.T) {
	h, _ := listQueryServer(t)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	for name, tc := range map[string]struct {
		tool string
		args map[string]any
	}{
		"number value":      {"list_time_entries", map[string]any{"extra_params": map[string]any{"cf_3": 5}}},
		"array of numbers":  {"list_issues", map[string]any{"extra_params": map[string]any{"f[]": []int{1}}}},
		"archived projects": {"list_projects", map[string]any{"status": "archived"}},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err == nil && (res == nil || !res.IsError) {
				t.Fatalf("%s accepted %v", tc.tool, tc.args)
			}
		})
	}
}
