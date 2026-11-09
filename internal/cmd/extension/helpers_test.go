package extension

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/ghrelease"
)

// fakeReleases serves GitHub releases and search for command tests.
type fakeReleases struct {
	srv *httptest.Server
	mu  sync.Mutex
	// latest maps "owner/repo" to the latest release tag and its assets.
	latest map[string]fakeRelease
	search []map[string]any
}

type fakeRelease struct {
	tag    string
	assets map[string][]byte
}

func newFakeReleases(t *testing.T) *fakeReleases {
	t.Helper()
	fr := &fakeReleases{latest: map[string]fakeRelease{}}
	fr.srv = httptest.NewServer(http.HandlerFunc(fr.serve))
	t.Cleanup(fr.srv.Close)
	return fr
}

func (fr *fakeReleases) set(repo, tag string, assets map[string][]byte) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.latest[repo] = fakeRelease{tag: tag, assets: assets}
}

func (fr *fakeReleases) serve(w http.ResponseWriter, r *http.Request) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.URL.Path == "/search/repositories":
		_ = json.NewEncoder(w).Encode(map[string]any{"items": fr.search})
	case len(parts) == 5 && parts[0] == "repos" && parts[3] == "releases" && parts[4] == "latest":
		rel, ok := fr.latest[parts[1]+"/"+parts[2]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		assets := []map[string]any{}
		for name, data := range rel.assets {
			assets = append(assets, map[string]any{
				"name": name, "size": len(data),
				"browser_download_url": fr.srv.URL + "/download/" + parts[1] + "/" + parts[2] + "/" + name,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": rel.tag, "assets": assets})
	case len(parts) == 4 && parts[0] == "download":
		rel, ok := fr.latest[parts[1]+"/"+parts[2]]
		if data, found := rel.assets[parts[3]]; ok && found {
			_, _ = w.Write(data)
			return
		}
		http.NotFound(w, r)
	default:
		http.NotFound(w, r)
	}
}

// testEnv is a root command with the extension commands and a Manager on a
// temp directory backed by the fake GitHub.
type testEnv struct {
	t       *testing.T
	f       *cmdutil.Factory
	opts    Options
	gh      *fakeReleases
	extRoot string
	stdin   *bytes.Buffer
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gh := newFakeReleases(t)
	extRoot := filepath.Join(t.TempDir(), "extensions")
	te := &testEnv{
		t:       t,
		gh:      gh,
		extRoot: extRoot,
		stdin:   &bytes.Buffer{},
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
	}
	te.opts = Options{
		Version: "2.9.0",
		NewManager: func() (*extmgr.Manager, error) {
			return &extmgr.Manager{
				Root:   extRoot,
				GitHub: &ghrelease.Client{HTTPClient: gh.srv.Client(), APIBaseURL: gh.srv.URL},
				GOOS:   "linux",
				GOARCH: "amd64",
			}, nil
		},
	}
	t.Setenv("REDMINE_NO_UPDATE_CHECK", "")
	return te
}

func (te *testEnv) manager() *extmgr.Manager {
	m, err := te.opts.NewManager()
	if err != nil {
		te.t.Fatal(err)
	}
	return m
}

// root builds a minimal root: the global flags the extension commands read,
// a core "issues" command, and `extension`.
func (te *testEnv) root() *cobra.Command {
	te.f = &cmdutil.Factory{
		ConfigPath: filepath.Join(te.t.TempDir(), "missing.yaml"),
		IOStreams:  &cmdutil.IOStreams{In: te.stdin, Out: te.stdout, ErrOut: te.stderr},
	}
	root := &cobra.Command{
		Use:           "redmine",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ApplyGlobalFlags(te.f, cmd)
		},
	}
	root.PersistentFlags().String("profile", "", "")
	root.PersistentFlags().String("config", "", "")
	root.PersistentFlags().StringP("output", "o", "", "")
	root.PersistentFlags().StringP("server", "s", "", "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.AddCommand(&cobra.Command{Use: "issues", Aliases: []string{"issue"}, Args: cobra.NoArgs, Run: func(*cobra.Command, []string) {}})
	root.AddCommand(NewCmdExtension(te.f, te.opts))
	return root
}

// run executes args against a fresh root.
func (te *testEnv) run(args ...string) error {
	te.t.Helper()
	te.stdout.Reset()
	te.stderr.Reset()
	root := te.root()
	root.SetArgs(args)
	root.SetIn(te.stdin)
	root.SetOut(te.stdout)
	root.SetErr(te.stderr)
	return root.ExecuteContext(WithInvocationArgs(te.t.Context(), args))
}

// setTerminal makes the prompts believe stdin is a terminal.
func setTerminal(t *testing.T, tty bool) {
	t.Helper()
	orig := stdinIsTerminal
	stdinIsTerminal = func(*cmdutil.Factory) bool { return tty }
	t.Cleanup(func() { stdinIsTerminal = orig })
}

func manifestFor(name string) string {
	return `schema_version: 1
name: ` + name + `
description: Test extension
commands:
  - name: list
    description: List things
    params:
      - { name: issue_id, type: integer, required: true, positional: true }
  - name: push
    description: Push things
    writes: true
    mcp: false
`
}

func writeYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
