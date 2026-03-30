package user

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/testutil"
)

const userJSON = `{"id":1,"login":"admin","admin":true,"firstname":"John","lastname":"Doe","mail":"john@example.com","created_on":"2025-01-01T00:00:00Z","last_login_on":"2025-06-15T08:00:00Z","status":1}`

func userListHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"users":[` + userJSON + `],"total_count":3}`))
}

// --- list ---

func TestUserList_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stdout(f); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q", got, "[]\n")
	}
}

func TestUserList_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(userListHandler))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserList(f)
	cmd.SetArgs([]string{"--output", "table", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"admin", "John Doe", "john@example.com"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestUserList_PaginationWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(userListHandler))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserList(f)
	cmd.SetArgs([]string{"--output", "table", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Showing 1 of 3 users") {
		t.Fatalf("stderr = %q, want pagination warning", stderr)
	}
}

func TestUserList_CSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[` + userJSON + `],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserList(f)
	cmd.SetArgs([]string{"--output", "csv"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"ID", "Login", "admin", "John Doe"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("csv output missing %q:\n%s", want, stdout)
		}
	}
}

// --- me ---

func TestUserMe_Detail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/current.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":` + userJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserMe(f)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"admin", "John Doe", "john@example.com"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("detail output missing %q:\n%s", want, stdout)
		}
	}
}

func TestUserMe_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":` + userJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactoryWithConfig(t, srv.URL, "output_format: json\n")
	cmd := newCmdUserMe(f)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, `"login": "admin"`) {
		t.Errorf("expected JSON output with login, got:\n%s", stdout)
	}
}
