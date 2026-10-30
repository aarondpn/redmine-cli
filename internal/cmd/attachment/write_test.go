package attachment

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

type seen struct{ method, uri, body string }

func writeServer(t *testing.T) (*httptest.Server, *[]seen) {
	t.Helper()
	var calls []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, seen{r.Method, r.URL.RequestURI(), string(b)})
		if strings.HasPrefix(r.URL.Path, "/attachments/thumbnail/") {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestCmdAttachmentUpdate(t *testing.T) {
	srv, calls := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"7", "--filename", "new.png", "--description", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.method != "PUT" || c.uri != "/attachments/7.json" || c.body != `{"attachment":{"filename":"new.png","description":""}}` {
		t.Errorf("call = %+v", c)
	}
	if out := testutil.Stdout(f) + testutil.Stderr(f); !strings.Contains(out, "updated") {
		t.Errorf("output = %q", out)
	}
}

func TestCmdAttachmentUpdate_RequiresAFlag(t *testing.T) {
	srv, calls := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"7"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error without any field flag")
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestCmdAttachmentDelete_Force(t *testing.T) {
	srv, calls := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdDelete(f)
	cmd.SetArgs([]string{"7", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0].method != "DELETE" || (*calls)[0].uri != "/attachments/7.json" {
		t.Errorf("calls = %v", *calls)
	}
}

func TestCmdAttachmentDelete_Cancelled(t *testing.T) {
	srv, calls := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdDelete(f)
	cmd.SetArgs([]string{"7"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Errorf("DELETE sent despite declined confirmation: %v", *calls)
	}
	if got := testutil.Stderr(f); !strings.Contains(got, "Delete cancelled") {
		t.Errorf("stderr = %q", got)
	}
}

func TestCmdAttachmentThumbnail_File(t *testing.T) {
	srv, calls := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	dest := filepath.Join(t.TempDir(), "t.png")
	cmd := newCmdThumbnail(f)
	cmd.SetArgs([]string{"7", "--path", dest, "--size", "120", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(pngBytes) {
		t.Errorf("file = %v", got)
	}
	if (*calls)[0].uri != "/attachments/thumbnail/7/120" {
		t.Errorf("uri = %q", (*calls)[0].uri)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{`"id": 7`, `"bytes": 12`, `"size": 120`, `"path"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json %q missing %q", out, want)
		}
	}
}

func TestCmdAttachmentThumbnail_DefaultSizePath(t *testing.T) {
	srv, calls := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdThumbnail(f)
	cmd.SetArgs([]string{"7", "--path", filepath.Join(t.TempDir(), "t.png")})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if (*calls)[0].uri != "/attachments/thumbnail/7" {
		t.Errorf("uri = %q", (*calls)[0].uri)
	}
}

func TestCmdAttachmentThumbnail_Stdout(t *testing.T) {
	srv, _ := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdThumbnail(f)
	cmd.SetArgs([]string{"7", "--path", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if testutil.Stdout(f) != string(pngBytes) {
		t.Errorf("stdout = %q, want raw bytes only", testutil.Stdout(f))
	}
	if !strings.Contains(testutil.Stderr(f), "to stdout") {
		t.Errorf("stderr = %q", testutil.Stderr(f))
	}
}

func TestCmdAttachmentThumbnail_NoClobber(t *testing.T) {
	srv, _ := writeServer(t)
	f := testutil.NewFactory(t, srv.URL)
	dest := filepath.Join(t.TempDir(), "t.png")
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newCmdThumbnail(f)
	cmd.SetArgs([]string{"7", "--path", dest})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--clobber") {
		t.Fatalf("err = %v, want clobber hint", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "old" {
		t.Errorf("file modified: %q", got)
	}

	f2 := testutil.NewFactory(t, srv.URL)
	cmd = newCmdThumbnail(f2)
	cmd.SetArgs([]string{"7", "--path", dest, "--clobber"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(pngBytes) {
		t.Errorf("file not replaced: %q", got)
	}
}

func TestCmdAttachmentThumbnail_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	f := testutil.NewFactory(t, srv.URL)
	dest := filepath.Join(t.TempDir(), "t.png")
	cmd := newCmdThumbnail(f)
	cmd.SetArgs([]string{"7", "--path", dest})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("destination file must not be created on failure")
	}
}

func TestCmdAttachmentThumbnail_Validation(t *testing.T) {
	srv, calls := writeServer(t)
	for name, args := range map[string][]string{
		"missing path":  {"7"},
		"negative size": {"7", "--path", "x", "--size", "-5"},
		"bad id":        {"abc", "--path", "x"},
	} {
		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdThumbnail(f)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}
