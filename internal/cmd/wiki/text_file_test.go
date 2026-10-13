package wiki

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const wikiLongText = "h1. Title\n\n<pre>\n$ echo \"quoted\" && exit 1\n</pre>\n\nÜmlaut ✓\n"

// newWikiPutServer answers the project lookup and records wiki PUT bodies.
func newWikiPutServer(t *testing.T) (*httptest.Server, func() []map[string]any) {
	t.Helper()
	var (
		mu   sync.Mutex
		puts []map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(b, &body)
			mu.Lock()
			puts = append(puts, body)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"wiki_page":{"title":"MyPage","version":1}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return puts
	}
}

func wikiText(t *testing.T, puts []map[string]any) any {
	t.Helper()
	if len(puts) != 1 {
		t.Fatalf("PUTs = %d, want 1", len(puts))
	}
	wp, _ := puts[0]["wiki_page"].(map[string]any)
	return wp["text"]
}

func TestCreate_TextFile(t *testing.T) {
	srv, puts := newWikiPutServer(t)
	path := filepath.Join(t.TempDir(), "page.textile")
	if err := os.WriteFile(path, []byte(wikiLongText), 0o600); err != nil {
		t.Fatal(err)
	}

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--text-file", path, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := wikiText(t, puts()); got != wikiLongText {
		t.Errorf("text = %q, want %q", got, wikiLongText)
	}
}

func TestCreate_RequiresTextOrTextFile(t *testing.T) {
	srv, puts := newWikiPutServer(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetArgs([]string{"MyPage", "--project", "proj"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "[text text-file]") {
		t.Fatalf("err = %v, want a one-required error naming text and text-file", err)
	}
	if len(puts()) != 0 {
		t.Error("no page should be written")
	}
}

func TestUpdate_TextFileFromStdinWithSection(t *testing.T) {
	srv, puts := newWikiPutServer(t)

	f := testutil.NewFactory(t, srv.URL)
	f.IOStreams.In.(*bytes.Buffer).WriteString(wikiLongText)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "-F", "-", "--section", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (--text-file must satisfy --section's --text requirement)", err)
	}
	if got := wikiText(t, puts()); got != wikiLongText {
		t.Errorf("text = %q, want %q", got, wikiLongText)
	}
}

func TestUpdate_TextAndTextFileMutuallyExclusive(t *testing.T) {
	srv, puts := newWikiPutServer(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--text", "x", "--text-file", "-"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "[text text-file] were all set") {
		t.Fatalf("err = %v, want a mutual-exclusion error", err)
	}
	if len(puts()) != 0 {
		t.Error("no page should be written")
	}
}
