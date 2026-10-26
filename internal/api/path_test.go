package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// unsafeIdentifiers are project identifiers that must never be sent: each one
// could address a different endpoint once a proxy or Redmine normalizes or
// decodes the path.
var unsafeIdentifiers = []string{
	"",
	".",
	"..",
	"../users/2",
	"%2e%2e",
	"..%2fusers",
	"%2F",
	"a/b",
	`a\b`,
	`..\users`,
	"foo?x=1",
	"foo#x",
	"foo\x00",
	"foo\nbar",
	"foo\tbar",
	"foo\x7f",
	"foo\u0085",
}

func TestPathSegment(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "identifier", in: "my-project", want: "my-project"},
		{name: "underscore", in: "proj_1", want: "proj_1"},
		{name: "numeric id", in: "123", want: "123"},
		{name: "uppercase", in: "Legacy", want: "Legacy"},
		{name: "space is escaped", in: "with space", want: "with%20space"},
		{name: "unicode is escaped", in: "café", want: "caf%C3%A9"},
		{name: "dots inside are kept", in: "a..b", want: "a..b"},
		{name: "plus is kept", in: "a+b", want: "a+b"},
	}
	for _, in := range unsafeIdentifiers {
		tests = append(tests, struct {
			name    string
			in      string
			want    string
			wantErr bool
		}{name: "reject " + in, in: in, wantErr: true})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pathSegment("project identifier", tt.in)
			if tt.wantErr {
				var segErr *InvalidPathSegmentError
				if !errors.As(err, &segErr) {
					t.Fatalf("pathSegment(%q) error = %v, want *InvalidPathSegmentError", tt.in, err)
				}
				if segErr.Value != tt.in || segErr.Kind != "project identifier" {
					t.Errorf("error = %+v, want Kind %q and Value %q", segErr, "project identifier", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("pathSegment(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("pathSegment(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestInvalidPathSegmentError_Message(t *testing.T) {
	_, err := projectSegment("../users/2")
	want := `invalid project identifier "../users/2": must not contain '/'`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

// recordingServer counts requests and records their escaped paths. It
// answers every request with a body that satisfies all project-scoped calls.
type recordingServer struct {
	*httptest.Server
	mu    sync.Mutex
	paths []string
}

func newRecordingServer(t *testing.T) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.paths = append(rs.paths, r.Method+" "+r.URL.EscapedPath())
		rs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"project": {"id": 1, "identifier": "p"},
			"membership": {"id": 1},
			"version": {"id": 1},
			"wiki_page": {"title": "Page"},
			"memberships": [], "issue_categories": [], "versions": [],
			"files": [], "wiki_pages": [], "results": [],
			"total_count": 0
		}`))
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *recordingServer) requests() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]string(nil), rs.paths...)
}

func newPathTestClient(ts *httptest.Server) *Client {
	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}
	c.Categories = &CategoryService{client: c}
	c.Memberships = &MembershipService{client: c}
	c.Versions = &VersionService{client: c}
	c.Files = &FileService{client: c}
	c.Wikis = &WikiService{client: c}
	c.Search = &SearchService{client: c}
	return c
}

// projectScopedCall is one API method that takes a project identifier, with
// the request it is expected to send for a legitimate identifier (formatted
// with the escaped identifier substituted for %s).
type projectScopedCall struct {
	name string
	want string
	call func(c *Client, id string) error
}

var projectScopedCalls = []projectScopedCall{
	{"Projects.Get", "GET /projects/%s.json", func(c *Client, id string) error {
		_, err := c.Projects.Get(context.Background(), id, nil)
		return err
	}},
	{"Projects.Update", "PUT /projects/%s.json", func(c *Client, id string) error {
		return c.Projects.Update(context.Background(), id, models.ProjectUpdate{})
	}},
	{"Projects.Delete", "DELETE /projects/%s.json", func(c *Client, id string) error {
		return c.Projects.Delete(context.Background(), id)
	}},
	{"Projects.Archive", "PUT /projects/%s/archive.json", func(c *Client, id string) error {
		return c.Projects.Archive(context.Background(), id)
	}},
	{"Projects.Unarchive", "PUT /projects/%s/unarchive.json", func(c *Client, id string) error {
		return c.Projects.Unarchive(context.Background(), id)
	}},
	{"Projects.Members", "GET /projects/%s/memberships.json", func(c *Client, id string) error {
		_, _, err := c.Projects.Members(context.Background(), id, 0, 0)
		return err
	}},
	{"Categories.List", "GET /projects/%s/issue_categories.json", func(c *Client, id string) error {
		_, _, err := c.Categories.List(context.Background(), id)
		return err
	}},
	{"Memberships.List", "GET /projects/%s/memberships.json", func(c *Client, id string) error {
		_, _, err := c.Memberships.List(context.Background(), id, 0, 0)
		return err
	}},
	{"Memberships.Create", "POST /projects/%s/memberships.json", func(c *Client, id string) error {
		_, err := c.Memberships.Create(context.Background(), id, models.MembershipCreate{})
		return err
	}},
	{"Versions.List", "GET /projects/%s/versions.json", func(c *Client, id string) error {
		_, _, err := c.Versions.List(context.Background(), id, 0, 0)
		return err
	}},
	{"Versions.ListFiltered", "GET /projects/%s/versions.json", func(c *Client, id string) error {
		_, _, err := c.Versions.ListFiltered(context.Background(), id, 0, func(models.Version) bool { return true })
		return err
	}},
	{"Versions.Create", "POST /projects/%s/versions.json", func(c *Client, id string) error {
		_, err := c.Versions.Create(context.Background(), id, models.VersionCreate{})
		return err
	}},
	{"Files.List", "GET /projects/%s/files.json", func(c *Client, id string) error {
		_, _, err := c.Files.List(context.Background(), id, 0, 0)
		return err
	}},
	{"Files.Create", "POST /projects/%s/files.json", func(c *Client, id string) error {
		return c.Files.Create(context.Background(), id, models.ProjectFileCreate{})
	}},
	{"Wikis.List", "GET /projects/%s/wiki/index.json", func(c *Client, id string) error {
		_, _, err := c.Wikis.List(context.Background(), id, 0, 0)
		return err
	}},
	{"Wikis.Get", "GET /projects/%s/wiki/Page.json", func(c *Client, id string) error {
		_, err := c.Wikis.Get(context.Background(), id, "Page", nil)
		return err
	}},
	{"Wikis.GetVersion", "GET /projects/%s/wiki/Page/2.json", func(c *Client, id string) error {
		_, err := c.Wikis.GetVersion(context.Background(), id, "Page", 2)
		return err
	}},
	{"Wikis.Create", "PUT /projects/%s/wiki/Page.json", func(c *Client, id string) error {
		_, err := c.Wikis.Create(context.Background(), id, "Page", models.WikiPageCreate{})
		return err
	}},
	{"Wikis.Update", "PUT /projects/%s/wiki/Page.json", func(c *Client, id string) error {
		return c.Wikis.Update(context.Background(), id, "Page", models.WikiPageUpdate{})
	}},
	{"Wikis.Delete", "DELETE /projects/%s/wiki/Page.json", func(c *Client, id string) error {
		return c.Wikis.Delete(context.Background(), id, "Page")
	}},
	{"Search.Search", "GET /projects/%s/search.json", func(c *Client, id string) error {
		_, _, err := c.Search.Search(context.Background(), SearchParams{Query: "q", ProjectID: id})
		return err
	}},
}

func TestProjectScopedCalls_RejectUnsafeIdentifiers(t *testing.T) {
	for _, tc := range projectScopedCalls {
		for _, id := range unsafeIdentifiers {
			// Search treats an empty project as "search everywhere".
			if tc.name == "Search.Search" && id == "" {
				continue
			}
			t.Run(tc.name+"/"+id, func(t *testing.T) {
				rs := newRecordingServer(t)
				c := newPathTestClient(rs.Server)

				err := tc.call(c, id)
				var segErr *InvalidPathSegmentError
				if !errors.As(err, &segErr) {
					t.Fatalf("error = %v, want *InvalidPathSegmentError", err)
				}
				if got := rs.requests(); len(got) != 0 {
					t.Errorf("unsafe identifier %q reached the server: %v", id, got)
				}
			})
		}
	}
}

func TestProjectScopedCalls_LegitimateIdentifiers(t *testing.T) {
	ids := []struct{ in, escaped string }{
		{"my-project", "my-project"},
		{"proj_1", "proj_1"},
		{"123", "123"},
		{"with space", "with%20space"},
	}
	for _, tc := range projectScopedCalls {
		for _, id := range ids {
			t.Run(tc.name+"/"+id.in, func(t *testing.T) {
				rs := newRecordingServer(t)
				c := newPathTestClient(rs.Server)

				if err := tc.call(c, id.in); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				got := rs.requests()
				want := fmt.Sprintf(tc.want, id.escaped)
				if len(got) != 1 || got[0] != want {
					t.Errorf("requests = %v, want exactly [%s]", got, want)
				}
			})
		}
	}
}

// TestProjectDelete_PathTraversalPoC reproduces the security audit PoC:
// DELETE /projects/../users/2.json must never be sent, not even to a server
// that would normalize it to /users/2.json.
func TestProjectDelete_PathTraversalPoC(t *testing.T) {
	var hits []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.EscapedPath())
		w.WriteHeader(http.StatusNoContent)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c := newPathTestClient(ts)
	for _, id := range []string{"../users/2", "..%2fusers%2f2", "%2e%2e/users/2"} {
		err := c.Projects.Delete(context.Background(), id)
		var segErr *InvalidPathSegmentError
		if !errors.As(err, &segErr) {
			t.Errorf("Delete(%q) error = %v, want *InvalidPathSegmentError", id, err)
		}
	}
	if len(hits) != 0 {
		t.Errorf("traversal requests reached the server: %v", hits)
	}
}

func TestWikiPageSegment(t *testing.T) {
	tests := []struct {
		page    string
		want    string
		wantErr bool
	}{
		{page: "Start_Page", want: "/projects/p/wiki/Start_Page.json"},
		{page: "a/b", want: "/projects/p/wiki/a%2Fb.json"},
		{page: "50%", want: "/projects/p/wiki/50%25.json"},
		{page: "q?x#y", want: "/projects/p/wiki/q%3Fx%23y.json"},
		{page: "Release 1.0", want: "/projects/p/wiki/Release%201.0.json"},
		{page: "a..b/..c", want: "/projects/p/wiki/a..b%2F..c.json"},
		{page: "", wantErr: true},
		{page: ".", wantErr: true},
		{page: "..", wantErr: true},
		{page: "x/../../../../users/2", wantErr: true},
		{page: `x\..\users`, wantErr: true},
		{page: "a/./b", wantErr: true},
		{page: "../", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.page, func(t *testing.T) {
			rs := newRecordingServer(t)
			c := newPathTestClient(rs.Server)

			_, err := c.Wikis.Get(context.Background(), "p", tt.page, nil)
			got := rs.requests()
			if tt.wantErr {
				var segErr *InvalidPathSegmentError
				if !errors.As(err, &segErr) {
					t.Fatalf("error = %v, want *InvalidPathSegmentError", err)
				}
				if len(got) != 0 {
					t.Errorf("dot-segment page %q reached the server: %v", tt.page, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != "GET "+tt.want {
				t.Errorf("requests = %v, want exactly [GET %s]", got, tt.want)
			}
		})
	}
}

// TestWikiDelete_EncodedSlashTraversal covers a proxy that decodes %2F before
// normalizing the path: a title such as "x/../../../../users/2" is escaped to
// a single segment by url.PathEscape, but would still collapse to
// /users/2.json there, so it must be rejected before any request is sent.
func TestWikiDelete_EncodedSlashTraversal(t *testing.T) {
	rs := newRecordingServer(t)
	c := newPathTestClient(rs.Server)

	for _, page := range []string{"x/../../../../users/2", `x\..\..\..\..\users\2`} {
		err := c.Wikis.Delete(context.Background(), "p", page)
		var segErr *InvalidPathSegmentError
		if !errors.As(err, &segErr) || segErr.Kind != "wiki page" {
			t.Errorf("Delete(%q) error = %v, want wiki page *InvalidPathSegmentError", page, err)
		}
	}
	if got := rs.requests(); len(got) != 0 {
		t.Errorf("traversal requests reached the server: %v", got)
	}
}
