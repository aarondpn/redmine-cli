package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// newSearchServer serves project lookups for "webapp" and "other" and records
// the path of the search request.
func newSearchServer(t *testing.T, searchedPath *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/projects/webapp.json":
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"webapp","name":"Web App"}}`))
		case "/projects/other.json":
			_, _ = w.Write([]byte(`{"project":{"id":2,"identifier":"other","name":"Other"}}`))
		case "/search.json", "/projects/webapp/search.json", "/projects/other/search.json":
			*searchedPath = r.URL.Path
			_, _ = w.Write([]byte(`{"results":[{"id":7,"title":"Bug #7: Login","type":"issue","datetime":"2026-01-02T00:00:00Z"}],"total_count":1}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

const defaultProjectNoticeFragment = `Searching project "webapp" (default project). Use --scope all to search all projects.`

func TestSearch_DefaultProjectNotice(t *testing.T) {
	commands := map[string]func(*cmdutil.Factory) *cobra.Command{
		"search":          NewCmdSearch,
		"search issues":   newCmdSearchIssues,
		"search wiki":     newCmdSearchWiki,
		"search news":     newCmdSearchNews,
		"search messages": newCmdSearchMessages,
	}

	cases := []struct {
		name       string
		config     string
		args       []string
		wantPath   string
		wantNotice string
	}{
		{
			name:       "default project applied implicitly",
			config:     "default_project: webapp\n",
			wantPath:   "/projects/webapp/search.json",
			wantNotice: defaultProjectNoticeFragment,
		},
		{
			name:       "subprojects scope keeps the default project",
			config:     "default_project: webapp\n",
			args:       []string{"--scope", "subprojects"},
			wantPath:   "/projects/webapp/search.json",
			wantNotice: `Searching project "webapp" and its subprojects (default project).`,
		},
		{
			name:     "explicit project is silent",
			config:   "default_project: webapp\n",
			args:     []string{"--project", "other"},
			wantPath: "/projects/other/search.json",
		},
		{
			name:     "scope all is silent",
			config:   "default_project: webapp\n",
			args:     []string{"--scope", "all"},
			wantPath: "/projects/webapp/search.json",
		},
		{
			name:       "unknown scope falls back to the project and keeps the notice",
			config:     "default_project: webapp\n",
			args:       []string{"--scope", "bogus"},
			wantPath:   "/projects/webapp/search.json",
			wantNotice: defaultProjectNoticeFragment,
		},
		{
			name:     "my_projects scope is silent",
			config:   "default_project: webapp\n",
			args:     []string{"--scope", "my_projects"},
			wantPath: "/projects/webapp/search.json",
		},
		{
			name:     "no default project is silent",
			wantPath: "/search.json",
		},
	}

	for cmdName, newCmd := range commands {
		for _, tc := range cases {
			t.Run(cmdName+"/"+tc.name, func(t *testing.T) {
				var searched string
				srv := newSearchServer(t, &searched)
				defer srv.Close()

				f := testutil.NewFactoryWithConfig(t, srv.URL, tc.config)
				cmd := newCmd(f)
				cmd.SetArgs(append([]string{"login", "--output", "json"}, tc.args...))
				if err := cmd.Execute(); err != nil {
					t.Fatalf("execute: %v", err)
				}

				if searched != tc.wantPath {
					t.Errorf("search path = %q, want %q", searched, tc.wantPath)
				}

				// stdout must stay machine-readable even when the notice fires.
				var results []map[string]any
				if err := json.Unmarshal([]byte(testutil.Stdout(f)), &results); err != nil {
					t.Fatalf("stdout is not JSON: %v\n%s", err, testutil.Stdout(f))
				}

				stderr := testutil.Stderr(f)
				if tc.wantNotice == "" {
					if strings.Contains(stderr, "default project") {
						t.Errorf("unexpected notice on stderr: %q", stderr)
					}
					return
				}
				if !strings.Contains(stderr, tc.wantNotice) {
					t.Errorf("stderr = %q, want notice containing %q", stderr, tc.wantNotice)
				}
			})
		}
	}
}

func TestSearch_DefaultProjectNoticeOnlyAfterSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/projects/webapp.json" {
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"webapp","name":"Web App"}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := testutil.NewFactoryWithConfig(t, srv.URL, "default_project: webapp\n")
	cmd := NewCmdSearch(f)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"login", "--output", "json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected search error")
	}
	if stderr := testutil.Stderr(f); strings.Contains(stderr, "default project") {
		t.Errorf("failed search should not print the notice, stderr = %q", stderr)
	}
}

func TestSearch_HelpDocumentsDefaultProject(t *testing.T) {
	f := testutil.NewFactory(t, "http://example.invalid")
	cmd := NewCmdSearch(f)
	if !strings.Contains(cmd.Long, "--scope all") {
		t.Errorf("search Long help should explain --scope all, got %q", cmd.Long)
	}
	for _, name := range []string{"project", "scope"} {
		usage := cmd.Flags().Lookup(name).Usage
		if !strings.Contains(usage, "default project") {
			t.Errorf("--%s usage should mention the default project, got %q", name, usage)
		}
	}
}
