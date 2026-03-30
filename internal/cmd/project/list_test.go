package project

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/testutil"
)

func TestCmdProjectList_CSVConfigDoesNotEmitANSI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projects":[{"id":1,"identifier":"demo","name":"Demo","status":1,"is_public":true}],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactoryWithConfig(t, srv.URL, "output_format: csv\n")
	f.IOStreams.IsTTY = true
	cmd := newCmdList(f)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stdout := testutil.Stdout(f)
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("csv output contains ANSI escapes:\n%q", stdout)
	}
	for _, want := range []string{"ID,Identifier,Name,Status,Public", "1,demo,Demo,active,yes"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("csv output missing %q:\n%s", want, stdout)
		}
	}
}
