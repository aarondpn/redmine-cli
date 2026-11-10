package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/ghrelease"
)

// fakeGitHub serves releases, asset downloads, commits and search like the
// GitHub API.
type fakeGitHub struct {
	t   *testing.T
	srv *httptest.Server

	mu sync.Mutex
	// releases maps "owner/repo" to releases, newest first; the first is
	// "latest".
	releases map[string][]fakeRelease
	commits  map[string]string
	search   []map[string]any
	// lastSearch is the q parameter of the last search.
	lastSearch string
	requests   []string
}

type fakeRelease struct {
	tag    string
	assets map[string][]byte
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	fg := &fakeGitHub{t: t, releases: map[string][]fakeRelease{}, commits: map[string]string{}}
	fg.srv = httptest.NewServer(http.HandlerFunc(fg.serve))
	t.Cleanup(fg.srv.Close)
	return fg
}

func (fg *fakeGitHub) client() *ghrelease.Client {
	return &ghrelease.Client{HTTPClient: fg.srv.Client(), APIBaseURL: fg.srv.URL}
}

func (fg *fakeGitHub) addRelease(repo, tag string, assets map[string][]byte) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.releases[repo] = append([]fakeRelease{{tag: tag, assets: assets}}, fg.releases[repo]...)
}

func (fg *fakeGitHub) requestCount(prefix string) int {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	n := 0
	for _, r := range fg.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (fg *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	fg.mu.Lock()
	defer fg.mu.Unlock()
	fg.requests = append(fg.requests, r.URL.Path)
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case len(parts) >= 4 && parts[0] == "repos" && parts[3] == "releases":
		repo := parts[1] + "/" + parts[2]
		rels := fg.releases[repo]
		var rel *fakeRelease
		switch {
		case len(parts) == 5 && parts[4] == "latest" && len(rels) > 0:
			rel = &rels[0]
		case len(parts) == 6 && parts[4] == "tags":
			for i := range rels {
				if rels[i].tag == parts[5] {
					rel = &rels[i]
				}
			}
		}
		if rel == nil {
			http.NotFound(w, r)
			return
		}
		type asset struct {
			Name string `json:"name"`
			Size int    `json:"size"`
			URL  string `json:"browser_download_url"`
		}
		out := struct {
			TagName string  `json:"tag_name"`
			Assets  []asset `json:"assets"`
		}{TagName: rel.tag, Assets: []asset{}}
		for name, data := range rel.assets {
			out.Assets = append(out.Assets, asset{Name: name, Size: len(data), URL: fg.srv.URL + "/download/" + repo + "/" + rel.tag + "/" + name})
		}
		_ = json.NewEncoder(w).Encode(out)
	case len(parts) == 4 && parts[0] == "repos" && parts[3] == "commits":
		sha, ok := fg.commits[parts[1]+"/"+parts[2]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{{"sha": sha}})
	case len(parts) == 5 && parts[0] == "download":
		repo := parts[1] + "/" + parts[2]
		for _, rel := range fg.releases[repo] {
			if rel.tag == parts[3] {
				if data, ok := rel.assets[parts[4]]; ok {
					_, _ = w.Write(data)
					return
				}
			}
		}
		http.NotFound(w, r)
	case r.URL.Path == "/search/repositories":
		fg.lastSearch = r.URL.Query().Get("q")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": fg.search})
	default:
		http.NotFound(w, r)
	}
}

// fakeGit fakes clones from a map of repository contents.
type fakeGit struct {
	mu sync.Mutex
	// repos maps clone URL to the files of the default branch; a file
	// whose name ends in "*" is written executable (without the "*").
	repos map[string]map[string]string
	// tags maps clone URL and tag ("url@tag") to files.
	tags map[string]map[string]string
	// upstream is the revision Fetch/Pull see, per clone dir base name.
	upstream string
	clones   []string
	pulls    int
	resets   int
	fail     error
}

func newFakeGit() *fakeGit {
	return &fakeGit{repos: map[string]map[string]string{}, tags: map[string]map[string]string{}, upstream: strings.Repeat("b", 40)}
}

const initialRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func (g *fakeGit) Clone(ctx context.Context, url, dir, ref string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.fail != nil {
		return g.fail
	}
	g.clones = append(g.clones, url+"@"+ref)
	files, ok := g.repos[url]
	if ref != "" {
		files, ok = g.tags[url+"@"+ref]
	}
	if !ok {
		return errors.New("repository not found")
	}
	for name, content := range files {
		perm := os.FileMode(0o644)
		if strings.HasSuffix(name, "*") {
			name, perm = strings.TrimSuffix(name, "*"), 0o755
		}
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), perm); err != nil {
			return err
		}
	}
	return writeHead(dir, initialRevision)
}

func writeHead(dir, rev string) error {
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte(rev+"\n"), 0o644)
}

func (g *fakeGit) Fetch(ctx context.Context, dir string) error { return nil }

func (g *fakeGit) Pull(ctx context.Context, dir string) error {
	g.mu.Lock()
	g.pulls++
	g.mu.Unlock()
	return writeHead(dir, g.upstream)
}

func (g *fakeGit) ResetHard(ctx context.Context, dir, rev string) error {
	g.mu.Lock()
	g.resets++
	g.mu.Unlock()
	if rev != "@{u}" {
		return fmt.Errorf("unexpected rev %q", rev)
	}
	return writeHead(dir, g.upstream)
}

func (g *fakeGit) Revision(ctx context.Context, dir, rev string) (string, error) {
	switch rev {
	case "HEAD":
		return readGitHead(dir)
	case "@{u}":
		return g.upstream, nil
	}
	return "", fmt.Errorf("unknown rev %q", rev)
}

// testManager returns a Manager on a temp root with fake GitHub and git.
func testManager(t *testing.T) (*Manager, *fakeGitHub, *fakeGit) {
	t.Helper()
	fg := newFakeGitHub(t)
	git := newFakeGit()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	m := &Manager{
		Root:       filepath.Join(t.TempDir(), "extensions"),
		CLIVersion: "2.9.0",
		GitHub:     fg.client(),
		Git:        git,
		CoreNames:  []string{"issues", "issue", "api"},
		GOOS:       "linux",
		GOARCH:     "amd64",
		Now:        func() time.Time { return now },
		CloneURL:   func(owner, repo string) string { return "fake://" + owner + "/" + repo },
	}
	return m, fg, git
}

// manifestFor returns a minimal valid manifest for extension name.
func manifestFor(name string) string {
	return `schema_version: 1
name: ` + name + `
description: Test extension
commands:
  - name: list
    description: List things
`
}
