package user

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
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

// --- get with includes ---

const userIncludesJSON = `{"id":1,"login":"admin","admin":true,"firstname":"John","lastname":"Doe","mail":"john@example.com","created_on":"2025-01-01T00:00:00Z","status":1,"mail_notification":"only_my_events","memberships":[{"id":42,"project":{"id":7,"name":"Apollo"},"roles":[{"id":3,"name":"Manager"}]}],"groups":[{"id":11,"name":"DevTeam"}]}`

// TestUserGet_IncludeQueryParam verifies the include flag is forwarded as a
// comma-joined query parameter and that memberships/groups are surfaced in
// detail output.
func TestUserGet_IncludeQueryParam(t *testing.T) {
	var (
		mu        sync.Mutex
		gotQuery  string
		gotPath   string
		callCount int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		gotQuery = r.URL.Query().Get("include")
		gotPath = r.URL.Path
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":` + userIncludesJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserGet(f)
	cmd.SetArgs([]string{"1", "--include", "memberships,groups"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if callCount != 1 {
		t.Fatalf("expected 1 server call, got %d", callCount)
	}
	if gotPath != "/users/1.json" {
		t.Errorf("path = %q, want /users/1.json", gotPath)
	}
	if gotQuery != "memberships,groups" {
		t.Errorf("include query = %q, want memberships,groups", gotQuery)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Apollo", "DevTeam", "Memberships", "Groups", "only_my_events"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("detail output missing %q:\n%s", want, stdout)
		}
	}
}

// TestUserGet_IncludeValidation rejects unknown include values without making
// any HTTP request.
func TestUserGet_IncludeValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be hit, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserGet(f)
	cmd.SetArgs([]string{"1", "--include", "nope"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown --include") {
		t.Errorf("error = %q, want it to mention unknown --include", err.Error())
	}
}

// TestUserMe_IncludePassThrough ensures --include reaches /users/current.json
// when used via `users me`.
func TestUserMe_IncludePassThrough(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/current.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotQuery = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":` + userIncludesJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserMe(f)
	cmd.SetArgs([]string{"--include", "groups"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "groups" {
		t.Errorf("include query = %q, want groups", gotQuery)
	}
}

// --- create with new flags ---

// TestUserCreate_NewFlags captures the POST body and asserts the new fields
// are nested inside the user object while send_information sits as a sibling.
func TestUserCreate_NewFlags(t *testing.T) {
	var (
		mu       sync.Mutex
		gotBody  map[string]interface{}
		gotCount int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/users.json" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(b, &gotBody)
		gotCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":` + userJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserCreate(f)
	cmd.SetArgs([]string{
		"--login", "newbie",
		"--password", "secret",
		"--firstname", "New",
		"--lastname", "Bie",
		"--mail", "new@example.com",
		"--mail-notification", "only_my_events",
		"--must-change-passwd",
		"--generate-password",
		"--auth-source-id", "7",
		"--send-information",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotCount != 1 {
		t.Fatalf("expected exactly one POST, got %d", gotCount)
	}
	user, ok := gotBody["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("body missing user key: %#v", gotBody)
	}
	if got := user["mail_notification"]; got != "only_my_events" {
		t.Errorf("user.mail_notification = %v, want only_my_events", got)
	}
	if got, _ := user["must_change_passwd"].(bool); !got {
		t.Errorf("user.must_change_passwd = %v, want true", user["must_change_passwd"])
	}
	if got, _ := user["generate_password"].(bool); !got {
		t.Errorf("user.generate_password = %v, want true", user["generate_password"])
	}
	if got, _ := user["auth_source_id"].(float64); int(got) != 7 {
		t.Errorf("user.auth_source_id = %v, want 7", user["auth_source_id"])
	}
	// send_information must be a sibling of user, not nested inside it.
	if _, nested := user["send_information"]; nested {
		t.Errorf("send_information must not be nested inside user object: %#v", user)
	}
	if got, _ := gotBody["send_information"].(bool); !got {
		t.Errorf("send_information sibling = %v, want true", gotBody["send_information"])
	}
}

// TestUserCreate_MailNotificationValidation short-circuits before the HTTP
// call when --mail-notification is invalid.
func TestUserCreate_MailNotificationValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be hit, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserCreate(f)
	cmd.SetArgs([]string{
		"--login", "x",
		"--password", "secret",
		"--firstname", "X",
		"--lastname", "Y",
		"--mail", "x@y.com",
		"--mail-notification", "bogus",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "mail-notification") {
		t.Errorf("error = %q, want mail-notification mention", err.Error())
	}
}

// TestUserCreate_RequiresPasswordOrGenerate verifies that calling create
// without either a password or --generate-password is rejected.
func TestUserCreate_RequiresPasswordOrGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be hit, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserCreate(f)
	cmd.SetArgs([]string{
		"--login", "x",
		"--firstname", "X",
		"--lastname", "Y",
		"--mail", "x@y.com",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("error = %q, want password mention", err.Error())
	}
}

// --- update with new flags ---

// TestUserUpdate_NewFlags captures the PUT body and asserts that only the
// fields whose flags were Changed are present, matching the existing
// firstname/lastname/mail/admin/status convention.
func TestUserUpdate_NewFlags(t *testing.T) {
	var (
		mu      sync.Mutex
		putBody map[string]interface{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Resolver fetches the user by ID before PUT to support 'me' and
			// name resolution. Reply with a minimal body.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user":` + userJSON + `}`))
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			_ = json.Unmarshal(b, &putBody)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserUpdate(f)
	cmd.SetArgs([]string{
		strconv.Itoa(1),
		"--mail-notification", "none",
		"--must-change-passwd",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	user, ok := putBody["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("body missing user key: %#v", putBody)
	}
	if got := user["mail_notification"]; got != "none" {
		t.Errorf("user.mail_notification = %v, want none", got)
	}
	if got, _ := user["must_change_passwd"].(bool); !got {
		t.Errorf("user.must_change_passwd = %v, want true", user["must_change_passwd"])
	}
	// Flags that were not Changed must be absent — pointer + omitempty is the
	// contract; a regression that defaulted them to zero would silently rewrite
	// the user's firstname/admin status.
	for _, omitted := range []string{"firstname", "lastname", "mail", "admin", "status", "generate_password", "auth_source_id"} {
		if _, present := user[omitted]; present {
			t.Errorf("user[%q] should be omitted when its flag was not set, got %v", omitted, user[omitted])
		}
	}
}
