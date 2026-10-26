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

// hiddenAttachmentServer serves attachment 56, whose server-supplied
// filename is ".bashrc".
func hiddenAttachmentServer(t *testing.T) *httptest.Server {
	t.Helper()
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
	return srv
}

// TestCmdAttachmentDownload_HiddenFilename refuses to save a server-supplied
// dotfile name under its own name in --dir and the default destination,
// naming the file and --allow-hidden; --allow-hidden and an explicit --path
// save it.
func TestCmdAttachmentDownload_HiddenFilename(t *testing.T) {
	tests := []struct {
		name    string
		args    func(dir string) ([]string, string)
		refused bool
	}{
		{"dir", func(dir string) ([]string, string) {
			return []string{"--dir", dir}, filepath.Join(dir, ".bashrc")
		}, true},
		{"default", func(string) ([]string, string) { return nil, ".bashrc" }, true},
		{"dir with --allow-hidden", func(dir string) ([]string, string) {
			return []string{"--dir", dir, "--allow-hidden"}, filepath.Join(dir, ".bashrc")
		}, false},
		{"default with --allow-hidden", func(string) ([]string, string) {
			return []string{"--allow-hidden"}, ".bashrc"
		}, false},
		{"path", func(dir string) ([]string, string) {
			p := filepath.Join(dir, ".bashrc")
			return []string{"--path", p}, p
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := hiddenAttachmentServer(t)
			f := testutil.NewFactory(t, srv.URL)
			dir := t.TempDir()
			t.Chdir(dir)
			flags, target := tt.args(dir)

			cmd := newCmdDownload(f)
			cmd.SetArgs(append([]string{"56", "--output", "json"}, flags...))
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			err := cmd.Execute()

			_, statErr := os.Stat(target)
			if !tt.refused {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if statErr != nil {
					t.Fatalf("expected the hidden file to be saved: %v", statErr)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), `".bashrc"`) || !strings.Contains(err.Error(), "--allow-hidden") {
				t.Fatalf("err = %v, want an error naming .bashrc and --allow-hidden", err)
			}
			if !os.IsNotExist(statErr) {
				t.Errorf("hidden file was saved (stat err = %v)", statErr)
			}
			if out := testutil.Stdout(f); out != "" {
				t.Errorf("stdout = %q, want nothing", out)
			}
		})
	}
}
