package issue

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

func TestResolveIssueAssigneeFilter_MeBypassesLookup(t *testing.T) {
	got, err := resolveIssueAssigneeFilter(context.Background(), nil, "me", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "me" {
		t.Fatalf("got %q, want %q", got, "me")
	}
}

func TestResolveIssueStatusFilter_PreservesSpecialValue(t *testing.T) {
	got, err := resolveIssueStatusFilter(context.Background(), nil, "*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "*" {
		t.Fatalf("got %q, want %q", got, "*")
	}
}

func TestResolveIssueAssigneeFilter_NumericBypassesLookup(t *testing.T) {
	got, err := resolveIssueAssigneeFilter(context.Background(), nil, "42", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "42" {
		t.Fatalf("got %q, want %q", got, "42")
	}
}

func TestResolveIssueAssigneeFilter_NoneMapsToUnassigned(t *testing.T) {
	got, err := resolveIssueAssigneeFilter(context.Background(), nil, "None", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "!*" {
		t.Fatalf("got %q, want %q", got, "!*")
	}
}

// forbiddenUsersClient returns a client whose /users.json and memberships
// endpoints are forbidden, like a non-admin key on a project whose members
// it cannot see.
func forbiddenUsersClient(t *testing.T) *api.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&config.Config{Server: srv.URL, APIKey: "k", AuthMethod: "apikey"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// Unresolvable names must fail the listing for both --assignee and --author
// instead of dropping the filter and returning unfiltered issues.
func TestResolveIssueUserFilters_PermissionErrorWithoutProjectFails(t *testing.T) {
	client := forbiddenUsersClient(t)
	for name, resolve := range map[string]func() (string, error){
		"assignee": func() (string, error) {
			return resolveIssueAssigneeFilter(context.Background(), client, "Jane Doe", "")
		},
		"author": func() (string, error) {
			return resolveIssueAuthorFilter(context.Background(), client, "Jane Doe", "")
		},
	} {
		got, err := resolve()
		if err == nil {
			t.Fatalf("%s: expected error, got filter %q", name, got)
		}
		for _, want := range []string{"resolving " + name, "admin privileges", `"me"`, "--project"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error %q missing %q", name, err, want)
			}
		}
	}
}

func TestResolveIssueUserFilters_PermissionErrorWithProjectFails(t *testing.T) {
	client := forbiddenUsersClient(t)
	_, err := resolveIssueAuthorFilter(context.Background(), client, "Jane Doe", "demo")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "Pass --project") {
		t.Fatalf("--project hint is pointless when a project is already set: %v", err)
	}
}
