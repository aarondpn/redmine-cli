package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

const demoMemberships = `{"memberships":[
	{"id":1,"project":{"id":7,"name":"Demo"},"user":{"id":5,"name":"Alice Smith"},"roles":[{"id":4,"name":"Developer"}]},
	{"id":2,"project":{"id":7,"name":"Demo"},"group":{"id":12,"name":"Backend Team"},"roles":[{"id":4,"name":"Developer"}]},
	{"id":3,"project":{"id":7,"name":"Demo"},"user":{"id":6,"name":"Bob Jones"},"roles":[{"id":4,"name":"Developer","inherited":true}]}
],"total_count":3,"offset":0,"limit":100}`

// principalServer is a fake Redmine that serves memberships for project
// "demo" (or 7) and either serves or forbids /users.json. It counts
// requests per path so tests can assert which endpoints were consulted.
type principalServer struct {
	mu          sync.Mutex
	hits        map[string]int
	memberships string
	membersCode int
	users       string
	usersCode   int
}

func (s *principalServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.hits[r.URL.Path]++
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/projects/demo/memberships.json", "/projects/7/memberships.json":
		if s.membersCode != 0 {
			w.WriteHeader(s.membersCode)
			return
		}
		_, _ = w.Write([]byte(s.memberships))
	case "/users.json":
		if s.usersCode != 0 {
			w.WriteHeader(s.usersCode)
			return
		}
		_, _ = w.Write([]byte(s.users))
	case "/users/current.json":
		_, _ = w.Write([]byte(`{"user":{"id":99,"login":"me","firstname":"Me","lastname":"Myself"}}`))
	default:
		http.NotFound(w, r)
	}
}

func (s *principalServer) count(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[path]
}

func newPrincipalServer(t *testing.T, srv *principalServer) *api.Client {
	t.Helper()
	srv.hits = map[string]int{}
	if srv.memberships == "" {
		srv.memberships = demoMemberships
	}
	if srv.users == "" {
		srv.users = `{"users":[],"total_count":0}`
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	client, err := api.NewClient(&config.Config{Server: ts.URL, APIKey: "k", AuthMethod: "apikey"}, debug.New(nil))
	if err != nil {
		t.Fatalf("api.NewClient: %v", err)
	}
	return client
}

func TestResolveAssigneeInProject_MatchesMemberUserWithoutAdmin(t *testing.T) {
	srv := &principalServer{usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	id, err := ResolveAssigneeInProject(context.Background(), client, "alice smith", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 5 {
		t.Fatalf("id = %d, want 5", id)
	}
	if n := srv.count("/users.json"); n != 0 {
		t.Fatalf("users endpoint hit %d times, want 0", n)
	}
}

func TestResolveAssigneeInProject_MatchesGroup(t *testing.T) {
	srv := &principalServer{usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	id, err := ResolveAssigneeInProject(context.Background(), client, "Backend Team", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 12 {
		t.Fatalf("id = %d, want 12", id)
	}
}

func TestResolveUserInProject_IgnoresGroups(t *testing.T) {
	srv := &principalServer{usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	_, err := ResolveUserInProject(context.Background(), client, "Backend Team", "demo")
	if err == nil {
		t.Fatal("expected error: groups are not users")
	}
	if strings.Contains(err.Error(), "Backend Team [group]") {
		t.Fatalf("user-only lookup should not suggest groups: %v", err)
	}
}

func TestResolveAssigneeInProject_NonAdminNoMatchListsMembers(t *testing.T) {
	srv := &principalServer{usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	_, err := ResolveAssigneeInProject(context.Background(), client, "jdoe", "demo")
	if err == nil {
		t.Fatal("expected error")
	}
	if IsNameResolutionPermissionError(err) {
		t.Fatalf("with a membership list the error must not be a silent-drop permission error: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"Available project members", "Alice Smith [user] (ID: 5)", "Backend Team [group] (ID: 12)", "admin privileges"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	if n := srv.count("/users.json"); n != 1 {
		t.Fatalf("users endpoint hit %d times, want 1 (login fallback)", n)
	}
}

func TestResolveAssigneeInProject_FallsBackToUsersForLogin(t *testing.T) {
	srv := &principalServer{users: `{"users":[{"id":5,"login":"asmith","firstname":"Alice","lastname":"Smith"}],"total_count":1}`}
	client := newPrincipalServer(t, srv)

	id, err := ResolveAssigneeInProject(context.Background(), client, "asmith", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 5 {
		t.Fatalf("id = %d, want 5", id)
	}
	if srv.count("/projects/demo/memberships.json") != 1 || srv.count("/users.json") != 1 {
		t.Fatalf("unexpected hits: %+v", srv.hits)
	}
}

func TestResolveAssigneeInProject_MembershipsForbiddenFallsBackToUsers(t *testing.T) {
	srv := &principalServer{
		membersCode: http.StatusForbidden,
		users:       `{"users":[{"id":5,"login":"asmith","firstname":"Alice","lastname":"Smith"}],"total_count":1}`,
	}
	client := newPrincipalServer(t, srv)

	id, err := ResolveAssigneeInProject(context.Background(), client, "Alice Smith", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 5 {
		t.Fatalf("id = %d, want 5", id)
	}
}

func TestResolveAssigneeInProject_BothForbiddenIsPermissionError(t *testing.T) {
	srv := &principalServer{membersCode: http.StatusForbidden, usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	_, err := ResolveAssigneeInProject(context.Background(), client, "Alice Smith", "demo")
	if !IsNameResolutionPermissionError(err) {
		t.Fatalf("expected NameResolutionPermissionError, got %v", err)
	}
}

func TestResolveAssignee_NoProjectNonAdminIsPermissionError(t *testing.T) {
	srv := &principalServer{usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	_, err := ResolveAssignee(context.Background(), client, "Alice Smith")
	if !IsNameResolutionPermissionError(err) {
		t.Fatalf("expected NameResolutionPermissionError, got %v", err)
	}
	if n := srv.count("/projects/demo/memberships.json"); n != 0 {
		t.Fatalf("memberships fetched without project context: %d", n)
	}
}

func TestResolveAssigneeInProject_AmbiguousUserAndGroup(t *testing.T) {
	srv := &principalServer{memberships: `{"memberships":[
		{"id":1,"project":{"id":7,"name":"Demo"},"user":{"id":5,"name":"Ops"},"roles":[]},
		{"id":2,"project":{"id":7,"name":"Demo"},"group":{"id":12,"name":"ops"},"roles":[]}
	],"total_count":2}`}
	client := newPrincipalServer(t, srv)

	_, err := ResolveAssigneeInProject(context.Background(), client, "OPS", "demo")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	msg := err.Error()
	for _, want := range []string{"multiple project members match", "Ops (user, ID: 5)", "ops (group, ID: 12)"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	if n := srv.count("/users.json"); n != 0 {
		t.Fatalf("ambiguity must not fall through to users endpoint, got %d hits", n)
	}
}

func TestResolveAssigneeInProject_NumericAndMeSkipLookups(t *testing.T) {
	srv := &principalServer{}
	client := newPrincipalServer(t, srv)

	if id, err := ResolveAssigneeInProject(context.Background(), client, "42", "demo"); err != nil || id != 42 {
		t.Fatalf("numeric: id=%d err=%v", id, err)
	}
	if id, err := ResolveAssigneeInProject(context.Background(), client, "me", "demo"); err != nil || id != 99 {
		t.Fatalf("me: id=%d err=%v", id, err)
	}
	if n := srv.count("/projects/demo/memberships.json"); n != 0 {
		t.Fatalf("memberships fetched for numeric/me input: %d", n)
	}
}

func TestResolveAssigneesInProject_FetchesMembershipsOnce(t *testing.T) {
	srv := &principalServer{usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	ids, err := ResolveAssigneesInProject(context.Background(), client, []string{"Alice Smith", "", "Backend Team", "Bob Jones", "7"}, "demo", "watcher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{5, 12, 6, 7}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
	if n := srv.count("/projects/demo/memberships.json"); n != 1 {
		t.Fatalf("memberships fetched %d times, want 1", n)
	}
}

func TestResolveAssigneesInProject_ErrorNamesInput(t *testing.T) {
	srv := &principalServer{usersCode: http.StatusForbidden}
	client := newPrincipalServer(t, srv)

	_, err := ResolveAssigneesInProject(context.Background(), client, []string{"nobody"}, "demo", "watcher")
	if err == nil || !strings.Contains(err.Error(), `resolving --watcher "nobody"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsNone(t *testing.T) {
	for _, in := range []string{"none", "NONE", " None "} {
		if !IsNone(in) {
			t.Errorf("IsNone(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"", "me", "nonexistent", "0"} {
		if IsNone(in) {
			t.Errorf("IsNone(%q) = true, want false", in)
		}
	}
}

func TestNeedsPrincipalLookup(t *testing.T) {
	cases := map[string]bool{
		"":             false,
		"me":           false,
		"ME":           false,
		"none":         false,
		"42":           false,
		"alice":        true,
		"Backend Team": true,
	}
	for in, want := range cases {
		if got := NeedsPrincipalLookup(in); got != want {
			t.Errorf("NeedsPrincipalLookup(%q) = %v, want %v", in, got, want)
		}
	}
}
