package project

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdProjectArchive_PutsToArchiveEndpoint(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdArchive(f)
	cmd.SetArgs([]string{"demo", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/projects/demo/archive.json" {
		t.Errorf("path = %s, want /projects/demo/archive.json", gotPath)
	}
	if !strings.Contains(testutil.Stderr(f), "archived") {
		t.Errorf("stderr missing archive message:\n%s", testutil.Stderr(f))
	}
}

func TestCmdProjectArchive_PromptCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called on cancel; got %s", r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	// Respond "n" to the confirmation prompt.
	f.IOStreams.In = bytes.NewBufferString("n\n")

	cmd := newCmdArchive(f)
	cmd.SetArgs([]string{"demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(testutil.Stderr(f), "cancelled") {
		t.Errorf("stderr missing cancellation message:\n%s", testutil.Stderr(f))
	}
}

func TestCmdProjectUnarchive_PutsToUnarchiveEndpoint(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUnarchive(f)
	cmd.SetArgs([]string{"demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/projects/demo/unarchive.json" {
		t.Errorf("path = %s, want /projects/demo/unarchive.json", gotPath)
	}
}
