package search

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestCmdSearchProjects_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testFactory(t, srv.URL)
	cmd := newCmdSearchProjects(f)
	cmd.SetArgs([]string{"nonexistent", "--output", "json"})

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

func TestCmdSearchProjects_EmptyTableWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testFactory(t, srv.URL)
	cmd := newCmdSearchProjects(f)
	cmd.SetArgs([]string{"nonexistent", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stderr := f.IOStreams.ErrOut.(*bytes.Buffer).String()
	if stderr == "" {
		t.Fatal("stderr is empty, want warning about no projects found")
	}
}
