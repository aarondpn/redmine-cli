package attachment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// pngBytes is a tiny payload containing NUL and other non-text bytes so the
// tests double as a binary-integrity check.
var pngBytes = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x01, 0x02, 0xff}

// attachmentServer serves metadata at /attachments/<id>.json and raw bytes at
// the content_url it advertises. id 55 -> logo.png.
func attachmentServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachments/55.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"attachment":{"id":55,"filename":"logo.png","filesize":%d,"content_type":"image/png","description":"the logo","content_url":%q,"author":{"id":1,"name":"Ada"},"created_on":"2026-01-01T00:00:00Z"}}`,
				len(pngBytes), srv.URL+"/attachments/download/55/logo.png")
		case "/attachments/download/55/logo.png":
			_, _ = w.Write(pngBytes)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCmdAttachmentGet_JSON(t *testing.T) {
	srv := attachmentServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"55", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	if !strings.Contains(out, `"filename": "logo.png"`) || !strings.Contains(out, `"content_type": "image/png"`) {
		t.Fatalf("json stdout = %q, want filename and content_type", out)
	}
}

func TestCmdAttachmentGet_TableAndCSV(t *testing.T) {
	for _, format := range []string{"table", "csv"} {
		t.Run(format, func(t *testing.T) {
			srv := attachmentServer(t)
			f := testutil.NewFactory(t, srv.URL)
			cmd := newCmdGet(f)
			cmd.SetArgs([]string{"55", "--output", format})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if out := testutil.Stdout(f); !strings.Contains(out, "logo.png") {
				t.Fatalf("%s stdout = %q, want logo.png", format, out)
			}
		})
	}
}

func TestCmdAttachmentGet_InvalidID(t *testing.T) {
	f := testutil.NewFactory(t, "http://127.0.0.1:0")
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"not-a-number"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}

func TestCmdAttachmentDownload_ToDirJSON(t *testing.T) {
	srv := attachmentServer(t)
	f := testutil.NewFactory(t, srv.URL)
	dir := t.TempDir()
	cmd := newCmdDownload(f)
	cmd.SetArgs([]string{"55", "--dir", dir, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var res struct {
		ID       int    `json:"id"`
		Filename string `json:"filename"`
		Path     string `json:"path"`
		Bytes    int64  `json:"bytes"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &res); err != nil {
		t.Fatalf("decode result: %v\nstdout: %s", err, testutil.Stdout(f))
	}
	wantPath := filepath.Join(dir, "logo.png")
	if res.Path != wantPath || res.Bytes != int64(len(pngBytes)) || res.Filename != "logo.png" {
		t.Fatalf("result = %+v, want path=%q bytes=%d", res, wantPath, len(pngBytes))
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != string(pngBytes) {
		t.Errorf("downloaded bytes differ from source (binary integrity)")
	}
}

func TestCmdAttachmentDownload_DefaultsToCurrentDir(t *testing.T) {
	srv := attachmentServer(t)
	f := testutil.NewFactory(t, srv.URL)
	t.Chdir(t.TempDir()) // bare `download <id>` saves into the working directory

	cmd := newCmdDownload(f)
	cmd.SetArgs([]string{"55"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("logo.png")
	if err != nil {
		t.Fatalf("expected ./logo.png in the working directory: %v", err)
	}
	if string(got) != string(pngBytes) {
		t.Error("downloaded bytes differ from source")
	}
}

func TestCmdAttachmentDownload_ToExplicitPath(t *testing.T) {
	srv := attachmentServer(t)
	f := testutil.NewFactory(t, srv.URL)
	dest := filepath.Join(t.TempDir(), "renamed.png")
	cmd := newCmdDownload(f)
	cmd.SetArgs([]string{"55", "--path", dest})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != string(pngBytes) {
		t.Error("downloaded bytes differ from source")
	}
	// Default (table) output writes the confirmation to stderr, never stdout.
	if testutil.Stdout(f) != "" {
		t.Errorf("stdout = %q, want empty (confirmation goes to stderr)", testutil.Stdout(f))
	}
	if !strings.Contains(testutil.Stderr(f), "renamed.png") {
		t.Errorf("stderr = %q, want confirmation mentioning the path", testutil.Stderr(f))
	}
}

func TestCmdAttachmentDownload_ToStdout(t *testing.T) {
	srv := attachmentServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdDownload(f)
	// --output json must NOT corrupt the piped binary on stdout.
	cmd.SetArgs([]string{"55", "--path", "-", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if testutil.Stdout(f) != string(pngBytes) {
		t.Errorf("stdout did not receive the raw bytes verbatim")
	}
	if !strings.Contains(testutil.Stderr(f), "stdout") {
		t.Errorf("stderr = %q, want confirmation mentioning stdout", testutil.Stderr(f))
	}
}

func TestCmdAttachmentDownload_DirAndPathMutuallyExclusive(t *testing.T) {
	f := testutil.NewFactory(t, "http://127.0.0.1:0")
	cmd := newCmdDownload(f)
	cmd.SetArgs([]string{"55", "--dir", "/tmp/x", "--path", "/tmp/y"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --dir and --path are combined")
	}
}
