package issue

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

type journalCall struct{ method, uri, body string }

func journalServer(t *testing.T, status int) (*httptest.Server, *[]journalCall) {
	t.Helper()
	var calls []journalCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, journalCall{r.Method, r.URL.RequestURI(), string(b)})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status >= 400 {
			_, _ = w.Write([]byte(`{"errors":["nope"]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func runEdit(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := newCmdCommentEdit(f)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestCommentDispatch(t *testing.T) {
	srv, calls := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)

	cmd := NewCmdComment(f)
	cmd.SetArgs([]string{"123", "-m", "hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = NewCmdComment(f)
	cmd.SetArgs([]string{"edit", "456", "-m", "changed"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls = %v", *calls)
	}
	if c := (*calls)[0]; c.method != "PUT" || c.uri != "/issues/123.json" {
		t.Errorf("parent call = %+v", c)
	}
	if c := (*calls)[1]; c.method != "PUT" || c.uri != "/journals/456.json" || c.body != `{"journal":{"notes":"changed"}}` {
		t.Errorf("child call = %+v", c)
	}
}

func TestCommentEdit_Message(t *testing.T) {
	srv, calls := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)
	if err := runEdit(t, f, "7", "-m", "new text", "--private"); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.body != `{"journal":{"notes":"new text","private_notes":true}}` {
		t.Errorf("body = %s", c.body)
	}
	if out := testutil.Stdout(f) + testutil.Stderr(f); !strings.Contains(out, "journal") {
		t.Errorf("output = %q", out)
	}
}

func TestCommentEdit_MessageFile(t *testing.T) {
	srv, calls := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)
	path := filepath.Join(t.TempDir(), "n.txt")
	if err := os.WriteFile(path, []byte("from file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runEdit(t, f, "7", "-F", path); err != nil {
		t.Fatal(err)
	}
	if (*calls)[0].body != `{"journal":{"notes":"from file"}}` {
		t.Errorf("body = %s", (*calls)[0].body)
	}
}

func TestCommentEdit_PublicSendsFalse(t *testing.T) {
	srv, calls := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)
	if err := runEdit(t, f, "7", "--public"); err != nil {
		t.Fatal(err)
	}
	if (*calls)[0].body != `{"journal":{"private_notes":false}}` {
		t.Errorf("body = %s", (*calls)[0].body)
	}
}

func TestCommentEdit_Validation(t *testing.T) {
	cases := map[string][]string{
		"no flags":          {"7"},
		"private+public":    {"7", "--private", "--public"},
		"clear+message":     {"7", "--clear", "-m", "x"},
		"empty message":     {"7", "-m", ""},
		"bad id":            {"abc", "-m", "x"},
		"zero id":           {"0", "-m", "x"},
		"message and -F":    {"7", "-m", "x", "-F", "-"},
		"missing file":      {"7", "-F", "/nonexistent/file"},
		"clear then public": {"7", "--clear", "--force", "--public", "--private"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			srv, calls := journalServer(t, http.StatusNoContent)
			f := testutil.NewFactory(t, srv.URL)
			if err := runEdit(t, f, args...); err == nil {
				t.Fatal("expected error")
			}
			if len(*calls) != 0 {
				t.Errorf("unexpected requests: %v", *calls)
			}
		})
	}
}

func TestCommentEdit_EmptyMessageHintsAtClear(t *testing.T) {
	srv, _ := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)
	err := runEdit(t, f, "7", "-m", "")
	if err == nil || !strings.Contains(err.Error(), "--clear") {
		t.Fatalf("err = %v", err)
	}
}

func TestCommentEdit_ClearForce(t *testing.T) {
	srv, calls := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)
	if err := runEdit(t, f, "7", "--clear", "--force"); err != nil {
		t.Fatal(err)
	}
	if (*calls)[0].body != `{"journal":{"notes":""}}` {
		t.Errorf("body = %s", (*calls)[0].body)
	}
}

func TestCommentEdit_ClearConfirmed(t *testing.T) {
	srv, calls := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)
	f.IOStreams.In = strings.NewReader("y\n")
	if err := runEdit(t, f, "7", "--clear"); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 {
		t.Errorf("calls = %v", *calls)
	}
}

func TestCommentEdit_ClearCancelled(t *testing.T) {
	srv, calls := journalServer(t, http.StatusNoContent)
	f := testutil.NewFactory(t, srv.URL)
	if err := runEdit(t, f, "7", "--clear"); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
	if out := testutil.Stdout(f) + testutil.Stderr(f); !strings.Contains(out, "Edit cancelled") {
		t.Errorf("output = %q", out)
	}
}

func TestCommentEdit_UnsupportedServerHint(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		srv, _ := journalServer(t, status)
		f := testutil.NewFactory(t, srv.URL)
		err := runEdit(t, f, "7", "-m", "x")
		if err == nil {
			t.Fatalf("status %d: expected error", status)
		}
		hint := cmdutil.ErrorHint(err)
		if !strings.Contains(hint, "5.0") {
			t.Errorf("status %d: hint = %q", status, hint)
		}
	}
}

func TestCommentEdit_ForbiddenHint(t *testing.T) {
	srv, _ := journalServer(t, http.StatusForbidden)
	f := testutil.NewFactory(t, srv.URL)
	err := runEdit(t, f, "7", "-m", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	hint := cmdutil.ErrorHint(err)
	if !strings.Contains(hint, "permission") || !strings.Contains(hint, "5.0") || !strings.Contains(hint, "retry") {
		t.Errorf("hint = %q", hint)
	}
}
