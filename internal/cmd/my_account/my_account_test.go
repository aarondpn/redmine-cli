package myaccount

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const myAccountJSON = `{"id":1,"login":"admin","admin":true,"firstname":"John","lastname":"Doe","mail":"john@example.com","created_on":"2025-01-01T00:00:00Z","last_login_on":"2025-06-15T08:00:00Z","status":1,"api_key":"deadbeef123","mail_notification":"only_my_events","custom_fields":[{"id":2,"name":"Department","value":"Engineering"}]}`

// TestMyAccount_GetDetail asserts the detail output surfaces api_key,
// mail_notification, and custom field values, all of which are unique to
// /my/account.json vs /users/current.json.
func TestMyAccount_GetDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/my/account.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":` + myAccountJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountGet(f)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stdout := testutil.Stdout(f)
	for _, want := range []string{"admin", "John Doe", "deadbeef123", "only_my_events", "Department: Engineering"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("detail output missing %q:\n%s", want, stdout)
		}
	}
}

// TestMyAccount_GetJSON confirms the JSON path emits the raw user object
// including api_key — scripts depend on that being present.
func TestMyAccount_GetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":` + myAccountJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactoryWithConfig(t, srv.URL, "output_format: json\n")
	cmd := newCmdMyAccountGet(f)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{`"login": "admin"`, `"api_key": "deadbeef123"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("json output missing %q:\n%s", want, stdout)
		}
	}
}

// TestMyAccount_Update verifies the PUT body shape: writable fields nested
// under user, untouched fields omitted, endpoint is /my/account.json.
func TestMyAccount_Update(t *testing.T) {
	var (
		mu      sync.Mutex
		gotPath string
		putBody map[string]interface{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotPath = r.URL.Path
		_ = json.Unmarshal(b, &putBody)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountUpdate(f)
	cmd.SetArgs([]string{
		"--firstname", "Updated",
		"--mail-notification", "only_my_events",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/my/account.json" {
		t.Errorf("path = %q, want /my/account.json", gotPath)
	}
	user, ok := putBody["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("body missing user key: %#v", putBody)
	}
	if got := user["firstname"]; got != "Updated" {
		t.Errorf("user.firstname = %v, want Updated", got)
	}
	if got := user["mail_notification"]; got != "only_my_events" {
		t.Errorf("user.mail_notification = %v, want only_my_events", got)
	}
	for _, omitted := range []string{"lastname", "mail"} {
		if _, present := user[omitted]; present {
			t.Errorf("user[%q] should be omitted when its flag was not set", omitted)
		}
	}
}

// TestMyAccount_UpdateMailNotificationValidation rejects invalid enum values
// before any HTTP traffic, mirroring the user-create behavior.
func TestMyAccount_UpdateMailNotificationValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be hit, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountUpdate(f)
	cmd.SetArgs([]string{"--mail-notification", "bogus"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mail-notification") {
		t.Errorf("error = %q, want mail-notification mention", err.Error())
	}
}
