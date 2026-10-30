package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// runStatusAgainst runs `auth status` for an apikey profile pointing at
// server and returns stdout and stderr.
func runStatusAgainst(t *testing.T, server, format string) (string, string) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "server: " + server + "\nauth_method: apikey\napi_key: some-key\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &cmdutil.Factory{
		ConfigPath:   cfgPath,
		OutputFormat: format,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}
	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return f.IOStreams.Out.(*strings.Builder).String(), f.IOStreams.ErrOut.(*strings.Builder).String()
}

func TestStatus_ProbeFailureReasons(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		wantReason    string
		wantAnonymous bool
		wantUser      string
		wantMessage   string
		wantHint      string
	}{
		{
			name:          "401 means credentials rejected",
			status:        http.StatusUnauthorized,
			wantReason:    "credentials_rejected",
			wantAnonymous: true,
			wantUser:      "authentication failed",
			wantMessage:   "rejected the configured credentials",
			wantHint:      "redmine auth login",
		},
		{
			name:          "403 means REST API disabled",
			status:        http.StatusForbidden,
			wantReason:    "rest_api_disabled",
			wantAnonymous: true,
			wantUser:      "authentication failed",
			wantMessage:   "REST API is disabled",
			wantHint:      "Enable REST web service",
		},
		{
			name:        "500 is a failed request, not rejected credentials",
			status:      http.StatusInternalServerError,
			wantReason:  "request_failed",
			wantUser:    "unavailable:",
			wantMessage: "Could not verify the credentials",
			wantHint:    "network connection",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			stdout, _ := runStatusAgainst(t, srv.URL, "json")
			var payload map[string]any
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("expected JSON output, got: %v\n%s", err, stdout)
			}
			if payload["active"] != false {
				t.Errorf("active = %v, want false", payload["active"])
			}
			if payload["reason"] != tt.wantReason {
				t.Errorf("reason = %v, want %q", payload["reason"], tt.wantReason)
			}
			if got, _ := payload["anonymous"].(bool); got != tt.wantAnonymous {
				t.Errorf("anonymous = %v, want %v", payload["anonymous"], tt.wantAnonymous)
			}
			if user, _ := payload["user"].(string); !strings.HasPrefix(user, tt.wantUser) {
				t.Errorf("user = %q, want prefix %q", user, tt.wantUser)
			}
			if msg, _ := payload["message"].(string); !strings.Contains(msg, tt.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", msg, tt.wantMessage)
			}
			if hint, _ := payload["hint"].(string); !strings.Contains(hint, tt.wantHint) {
				t.Errorf("hint = %q, want it to contain %q", hint, tt.wantHint)
			}
			if payload["server"] != srv.URL {
				t.Errorf("server = %v, want %q (existing fields stay)", payload["server"], srv.URL)
			}

			// Human output: the reason lands in the detail table and the
			// explanation on stderr.
			out, errOut := runStatusAgainst(t, srv.URL, "")
			if !strings.Contains(out, "Reason") || !strings.Contains(out, tt.wantReason) {
				t.Errorf("table output should show the reason %q, got:\n%s", tt.wantReason, out)
			}
			if !strings.Contains(errOut, tt.wantMessage) {
				t.Errorf("stderr should explain the failure (%q), got:\n%s", tt.wantMessage, errOut)
			}
		})
	}
}

func TestStatus_UnreachableServerIsNotRejectedCredentials(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	stdout, _ := runStatusAgainst(t, url, "json")
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected JSON output, got: %v", err)
	}
	if payload["reason"] != "request_failed" {
		t.Errorf("reason = %v, want request_failed", payload["reason"])
	}
	if _, ok := payload["anonymous"]; ok {
		t.Errorf("anonymous must not be reported for a network failure: %v", payload)
	}
}

func TestStatus_SuccessHasNoFailureFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":1,"login":"jdoe","firstname":"John","lastname":"Doe"}}`))
	}))
	defer srv.Close()

	stdout, _ := runStatusAgainst(t, srv.URL, "json")
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected JSON output, got: %v", err)
	}
	if payload["active"] != true || payload["user"] != "John Doe (jdoe)" {
		t.Errorf("payload = %v, want active user", payload)
	}
	for _, key := range []string{"reason", "hint", "message", "anonymous"} {
		if _, ok := payload[key]; ok {
			t.Errorf("unexpected %q on success: %v", key, payload)
		}
	}
}
