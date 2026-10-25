package attachment

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// TestCmdAttachmentDownload_ExistingFile covers the overwrite policy for every
// destination form: an existing file is kept (and the command fails, naming
// the path and --clobber) unless --clobber is given.
func TestCmdAttachmentDownload_ExistingFile(t *testing.T) {
	tests := []struct {
		name string
		// args builds the destination flags for the given directory and
		// returns the path the download targets.
		args func(dir string) ([]string, string)
	}{
		{"path", func(dir string) ([]string, string) {
			p := filepath.Join(dir, "renamed.png")
			return []string{"--path", p}, p
		}},
		{"dir", func(dir string) ([]string, string) {
			return []string{"--dir", dir}, filepath.Join(dir, "logo.png")
		}},
		{"default", func(dir string) ([]string, string) {
			return nil, "logo.png"
		}},
	}
	for _, tt := range tests {
		for _, clobber := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/clobber=%v", tt.name, clobber), func(t *testing.T) {
				srv := attachmentServer(t)
				f := testutil.NewFactory(t, srv.URL)
				dir := t.TempDir()
				t.Chdir(dir)
				flags, target := tt.args(dir)
				if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
					t.Fatal(err)
				}

				args := append([]string{"55"}, flags...)
				if clobber {
					args = append(args, "--clobber")
				}
				cmd := newCmdDownload(f)
				cmd.SetArgs(args)
				cmd.SilenceUsage, cmd.SilenceErrors = true, true
				err := cmd.Execute()

				got, readErr := os.ReadFile(target)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if clobber {
					if err != nil {
						t.Fatalf("unexpected error with --clobber: %v", err)
					}
					if string(got) != string(pngBytes) {
						t.Error("file was not replaced with the attachment bytes")
					}
					return
				}
				if err == nil {
					t.Fatal("expected an error for an existing destination")
				}
				if !strings.Contains(err.Error(), target) || !strings.Contains(err.Error(), "--clobber") {
					t.Errorf("error %q should name %s and suggest --clobber", err, target)
				}
				if string(got) != "original" {
					t.Errorf("existing file was modified: %q", got)
				}
			})
		}
	}
}

// TestCmdAttachmentDownload_HiddenFilenameWarns saves a server-supplied dotfile
// name but warns about it on stderr.
func TestCmdAttachmentDownload_HiddenFilenameWarns(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachments/56.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"attachment":{"id":56,"filename":".bashrc","filesize":4,"content_type":"text/plain","content_url":%q}}`,
				srv.URL+"/attachments/download/56/.bashrc")
		case "/attachments/download/56/.bashrc":
			_, _ = w.Write([]byte("evil"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	f := testutil.NewFactory(t, srv.URL)
	dir := t.TempDir()
	cmd := newCmdDownload(f)
	cmd.SetArgs([]string{"56", "--dir", dir, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".bashrc")); err != nil {
		t.Fatalf("expected the hidden file to be saved: %v", err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "hidden filename") || !strings.Contains(stderr, ".bashrc") {
		t.Errorf("stderr = %q, want a hidden-filename warning", stderr)
	}
	if strings.Contains(testutil.Stdout(f), "hidden") {
		t.Errorf("warning leaked to stdout: %q", testutil.Stdout(f))
	}
}
