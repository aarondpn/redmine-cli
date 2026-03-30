package project

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func testFactory(t *testing.T, serverURL string) *cmdutil.Factory {
	t.Helper()

	cfgPath := t.TempDir() + "/config.yaml"
	cfg := "server: " + serverURL + "\napi_key: test\nauth_method: apikey\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	return &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     &bytes.Buffer{},
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
			IsTTY:  false,
		},
	}
}

func TestCmdMembers_JSONEmptyDoesNotWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo/memberships.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memberships":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testFactory(t, srv.URL)
	cmd := newCmdMembers(f)
	cmd.SetArgs([]string{"demo", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if got := f.IOStreams.Out.(*bytes.Buffer).String(); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q", got, "[]\n")
	}
	if got := f.IOStreams.ErrOut.(*bytes.Buffer).String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestCmdMembers_JSONPaginatedDoesNotWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo/memberships.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "1" {
			t.Fatalf("limit = %q, want 1", got)
		}
		if got := r.URL.Query().Get("offset"); got != "0" {
			t.Fatalf("offset = %q, want 0", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memberships":[{"id":1,"project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"roles":[{"id":3,"name":"Manager"}]}],"total_count":2}`))
	}))
	defer srv.Close()

	f := testFactory(t, srv.URL)
	cmd := newCmdMembers(f)
	cmd.SetArgs([]string{"demo", "--output", "json", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stdout := f.IOStreams.Out.(*bytes.Buffer).String()
	if !strings.Contains(stdout, `"name": "Alice"`) {
		t.Fatalf("stdout missing membership payload:\n%s", stdout)
	}
	if got := f.IOStreams.ErrOut.(*bytes.Buffer).String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}
