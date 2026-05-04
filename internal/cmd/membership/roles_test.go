package membership

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

func TestResolveRoleIDs_RequiresOneFlagSource(t *testing.T) {
	client := newMembershipTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))

	_, err := resolveRoleIDs(context.Background(), client, nil, nil, false, false)
	if err == nil || !strings.Contains(err.Error(), "either --role-ids or --roles is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRoleIDs_RejectsMixedFlags(t *testing.T) {
	client := newMembershipTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))

	_, err := resolveRoleIDs(context.Background(), client, []int{1}, []string{"Manager"}, true, true)
	if err == nil || !strings.Contains(err.Error(), "either --role-ids or --roles") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRoleIDs_ResolvesNamesAndDeduplicates(t *testing.T) {
	client := newMembershipTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/roles.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roles":[{"id":1,"name":"Manager"},{"id":2,"name":"Developer"}]}`))
	}))

	got, err := resolveRoleIDs(context.Background(), client, nil, []string{"Manager", "2", "Manager"}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("resolved role IDs = %v, want [1 2]", got)
	}
}

func newMembershipTestClient(t *testing.T, handler http.Handler) *api.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := api.NewClient(&config.Config{
		Server:     srv.URL,
		APIKey:     "test",
		AuthMethod: "apikey",
	}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	return client
}
