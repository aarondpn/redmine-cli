package extension

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/ghrelease"
)

func TestDefaultRoot(t *testing.T) {
	home := func() (string, error) { return "/home/u", nil }
	noHome := func() (string, error) { return "", errors.New("no home") }
	tests := []struct {
		name    string
		goos    string
		env     map[string]string
		home    func() (string, error)
		want    string
		wantErr bool
	}{
		{name: "override", goos: "linux", env: map[string]string{DirEnv: "/tmp/ext", "XDG_DATA_HOME": "/xdg"}, home: home, want: "/tmp/ext"},
		{name: "xdg", goos: "linux", env: map[string]string{"XDG_DATA_HOME": "/xdg"}, home: home, want: filepath.Join("/xdg", "redmine-cli", "extensions")},
		{name: "relative xdg ignored", goos: "linux", env: map[string]string{"XDG_DATA_HOME": "rel"}, home: home, want: filepath.Join("/home/u", ".local", "share", "redmine-cli", "extensions")},
		{name: "home", goos: "darwin", home: home, want: filepath.Join("/home/u", ".local", "share", "redmine-cli", "extensions")},
		{name: "windows local appdata", goos: "windows", env: map[string]string{"LOCALAPPDATA": "/appdata", "XDG_DATA_HOME": "/xdg"}, home: home, want: filepath.Join("/appdata", "redmine-cli", "extensions")},
		{name: "no home", goos: "linux", home: noHome, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := defaultRoot(tt.goos, func(k string) string { return tt.env[k] }, tt.home)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), DirEnv) {
					t.Fatalf("err = %v, want a hint at %s", err, DirEnv)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestParseSource(t *testing.T) {
	cwd, _ := os.Getwd()
	tests := []struct {
		arg     string
		want    Source
		wantErr string
	}{
		{arg: "octo/redmine-cli-pr", want: Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}},
		{arg: "https://github.com/octo/redmine-cli-pr", want: Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}},
		{arg: "https://github.com/octo/redmine-cli-pr.git/", want: Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}},
		{arg: "https://GitHub.com/octo/redmine-cli-pr", want: Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}},
		{arg: "http://github.com/octo/redmine-cli-pr", wantErr: "only https://github.com"},
		{arg: "https://gitlab.com/octo/redmine-cli-pr", wantErr: "only https://github.com"},
		{arg: "https://user@github.com/octo/redmine-cli-pr", wantErr: "only https://github.com"},
		{arg: "octo/pr", wantErr: "not an extension"},
		{arg: "octo", wantErr: "use <owner>/redmine-cli-<name>"},
		{arg: "octo/redmine-cli-pr/extra", wantErr: "use <owner>/redmine-cli-<name>"},
		{arg: "../redmine-cli-local", want: Source{Name: "local", LocalDir: filepath.Join(filepath.Dir(cwd), "redmine-cli-local")}},
		{arg: ".", wantErr: "must be named redmine-cli-<name>"},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			got, err := ParseSource(tt.arg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	m := &Manager{CoreNames: []string{"issues", "issue"}}
	tests := []struct {
		name    string
		wantErr string
	}{
		{name: "pr"},
		{name: "my-tool2"},
		{name: "issues", wantErr: "core command"},
		{name: "issue", wantErr: "core command"},
		{name: "extension", wantErr: "reserved"},
		{name: "ext", wantErr: "reserved"},
		{name: "Bad", wantErr: "invalid"},
		{name: strings.Repeat("a", 33), wantErr: "longer than 32"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.ValidateName(tt.name)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func checksumsFor(files map[string][]byte) []byte {
	var b strings.Builder
	for name, data := range files {
		b.WriteString(ghrelease.SHA256Hex(data) + "  " + name + "\n")
	}
	return []byte(b.String())
}

func TestInstallBinary(t *testing.T) {
	bin := []byte("#!/bin/sh\necho binary\n")
	manifest := []byte(manifestFor("pr"))
	asset := BinaryAssetName("pr", "linux", "amd64")

	tests := []struct {
		name      string
		assets    map[string][]byte
		pin       string
		wantErr   string
		wantMan   bool
		wantPin   bool
		wantClone bool
	}{
		{name: "binary without checksums", assets: map[string][]byte{asset: bin}},
		{name: "binary with checksums and manifest", assets: map[string][]byte{
			asset: bin, spec.FileName: manifest, "checksums.txt": checksumsFor(map[string][]byte{asset: bin, spec.FileName: manifest}),
		}, wantMan: true},
		{name: "checksum mismatch", assets: map[string][]byte{
			asset: bin, "checksums.txt": checksumsFor(map[string][]byte{asset: []byte("other")}),
		}, wantErr: "checksum verification"},
		{name: "binary missing from checksums", assets: map[string][]byte{
			asset: bin, "redmine-cli-pr_checksums.txt": checksumsFor(map[string][]byte{"x": bin}),
		}, wantErr: "checksum verification"},
		{name: "manifest checksum mismatch", assets: map[string][]byte{
			asset: bin, spec.FileName: manifest, "SHA256SUMS": checksumsFor(map[string][]byte{asset: bin, spec.FileName: []byte("x")}),
		}, wantErr: "checksum verification of redmine-extension.yml"},
		{name: "invalid manifest", assets: map[string][]byte{asset: bin, spec.FileName: []byte(manifestFor("other"))}, wantErr: "invalid redmine-extension.yml"},
		{name: "no binary for this platform", assets: map[string][]byte{BinaryAssetName("pr", "darwin", "arm64"): bin}, wantErr: "no binary for linux/amd64"},
		{name: "pinned", assets: map[string][]byte{asset: bin}, pin: "v1.0.0", wantPin: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fg, _ := testManager(t)
			fg.addRelease("octo/redmine-cli-pr", "v1.0.0", tt.assets)
			if tt.pin != "" {
				// A newer latest release must not be picked when pinned.
				fg.addRelease("octo/redmine-cli-pr", "v2.0.0", map[string][]byte{asset: []byte("new")})
			}
			src := Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}
			ext, err := m.Install(context.Background(), src, InstallOptions{Pin: tt.pin})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				assertNoLeftovers(t, m)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ext.Kind != KindBinary || ext.Version() != "v1.0.0" || ext.Repo() != "octo/redmine-cli-pr" || ext.Err != nil {
				t.Fatalf("ext = %+v", ext)
			}
			if ext.Pinned() != tt.wantPin {
				t.Errorf("pinned = %v, want %v", ext.Pinned(), tt.wantPin)
			}
			if (ext.Manifest != nil) != tt.wantMan {
				t.Errorf("manifest = %v, want present %v", ext.Manifest, tt.wantMan)
			}
			data, err := os.ReadFile(ext.Path)
			if err != nil || string(data) != string(bin) {
				t.Fatalf("binary = %q, %v", data, err)
			}
			if runtime.GOOS != "windows" {
				if info, _ := os.Stat(ext.Path); info.Mode().Perm()&0o100 == 0 {
					t.Errorf("binary mode %v is not executable", info.Mode())
				}
			}
			if ext.Install.SHA256 != ghrelease.SHA256Hex(bin) || ext.Install.InstalledAt.IsZero() {
				t.Errorf("install info = %+v", ext.Install)
			}
			assertNoLeftovers(t, m)
		})
	}
}

// assertNoLeftovers checks that no staging directory survived.
func assertNoLeftovers(t *testing.T, m *Manager) {
	t.Helper()
	entries, _ := os.ReadDir(m.Root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") || strings.HasPrefix(e.Name(), ".old-") {
			t.Errorf("leftover %s in %s", e.Name(), m.Root)
		}
	}
}

func TestInstallAlreadyInstalledAndForce(t *testing.T) {
	m, fg, _ := testManager(t)
	asset := BinaryAssetName("pr", "linux", "amd64")
	fg.addRelease("octo/redmine-cli-pr", "v1.0.0", map[string][]byte{asset: []byte("one")})
	src := Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}
	if _, err := m.Install(context.Background(), src, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	var already *AlreadyInstalledError
	if _, err := m.Install(context.Background(), src, InstallOptions{}); !errors.As(err, &already) {
		t.Fatalf("err = %v, want AlreadyInstalledError", err)
	}
	fg.addRelease("octo/redmine-cli-pr", "v1.1.0", map[string][]byte{asset: []byte("two")})
	ext, err := m.Install(context.Background(), src, InstallOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(ext.Path); string(data) != "two" || ext.Version() != "v1.1.0" {
		t.Fatalf("after force: %q %s", data, ext.Version())
	}
	assertNoLeftovers(t, m)
}

func TestInstallRejectsCoreName(t *testing.T) {
	m, fg, _ := testManager(t)
	_, err := m.Install(context.Background(), Source{Owner: "o", Repo: "redmine-cli-issues", Name: "issues"}, InstallOptions{})
	if err == nil || !strings.Contains(err.Error(), "core command") {
		t.Fatalf("err = %v", err)
	}
	if n := fg.requestCount("/"); n != 0 {
		t.Fatalf("%d requests before the name check", n)
	}
}

func TestInstallGit(t *testing.T) {
	script := "#!/bin/sh\necho hi\n"
	tests := []struct {
		name     string
		files    map[string]string
		tagFiles map[string]string
		release  map[string][]byte
		pin      string
		wantErr  string
		wantMan  bool
	}{
		{name: "script without manifest", files: map[string]string{"redmine-cli-sh*": script}},
		{name: "script with manifest", files: map[string]string{"redmine-cli-sh*": script, spec.FileName: manifestFor("sh")}, wantMan: true},
		{name: "release without binaries falls back to git", files: map[string]string{"redmine-cli-sh*": script}, release: map[string][]byte{"notes.txt": []byte("x")}},
		{name: "pinned tag", tagFiles: map[string]string{"redmine-cli-sh*": script}, pin: "v0.1.0"},
		{name: "missing executable", files: map[string]string{"README.md": "x"}, wantErr: "executable redmine-cli-sh not found"},
		{name: "not executable", files: map[string]string{"redmine-cli-sh": script}, wantErr: "not executable"},
		{name: "invalid manifest", files: map[string]string{"redmine-cli-sh*": script, spec.FileName: "schema_version: 2\n"}, wantErr: "invalid redmine-extension.yml"},
		{name: "clone failure", wantErr: "repository not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && tt.name == "not executable" {
				t.Skip("no executable bit on Windows")
			}
			m, fg, git := testManager(t)
			if tt.files != nil {
				git.repos["fake://octo/redmine-cli-sh"] = tt.files
			}
			if tt.tagFiles != nil {
				git.tags["fake://octo/redmine-cli-sh@"+tt.pin] = tt.tagFiles
			}
			if tt.release != nil {
				fg.addRelease("octo/redmine-cli-sh", "v1.0.0", tt.release)
			}
			src := Source{Owner: "octo", Repo: "redmine-cli-sh", Name: "sh"}
			ext, err := m.Install(context.Background(), src, InstallOptions{Pin: tt.pin})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if _, err := os.Lstat(m.entryPath("sh")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("failed install left %s behind", m.entryPath("sh"))
				}
				if _, err := os.Stat(m.installMetaPath("sh")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("failed install left metadata behind")
				}
				assertNoLeftovers(t, m)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ext.Kind != KindGit || ext.Err != nil || ext.Repo() != "octo/redmine-cli-sh" {
				t.Fatalf("ext = %+v (err %v)", ext, ext.Err)
			}
			if (ext.Manifest != nil) != tt.wantMan {
				t.Errorf("manifest present = %v, want %v", ext.Manifest != nil, tt.wantMan)
			}
			if tt.pin != "" {
				if !ext.Pinned() || ext.Version() != tt.pin {
					t.Errorf("pinned=%v version=%q", ext.Pinned(), ext.Version())
				}
			} else if ext.Version() != initialRevision[:7] {
				t.Errorf("version = %q, want the short HEAD", ext.Version())
			}
			// Metadata lives outside the clone.
			if _, err := os.Stat(filepath.Join(ext.Dir, "install.yml")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("install metadata inside the clone")
			}
			if _, err := os.Stat(m.installMetaPath("sh")); err != nil {
				t.Errorf("git metadata missing: %v", err)
			}
		})
	}
}

func TestInstallLocal(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			m, _, _ := testManager(t)
			m.GOOS = goos
			dir := filepath.Join(t.TempDir(), "redmine-cli-dev")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "redmine-cli-dev"), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			src, err := ParseSource(dir)
			if err != nil {
				t.Fatal(err)
			}
			ext, err := m.Install(context.Background(), src, InstallOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if ext.Kind != KindLocal || ext.Err != nil || ext.Manifest != nil {
				t.Fatalf("ext = %+v", ext)
			}
			wantDir, _ := filepath.EvalSymlinks(dir)
			gotDir, _ := filepath.EvalSymlinks(ext.Dir)
			if gotDir != wantDir {
				t.Fatalf("dir = %q, want %q", gotDir, wantDir)
			}
			info, err := os.Lstat(m.entryPath("dev"))
			if err != nil {
				t.Fatal(err)
			}
			if goos == "windows" && !info.Mode().IsRegular() {
				t.Errorf("windows link is %v, want a link file", info.Mode())
			}
			if goos != "windows" && info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("link is %v, want a symlink", info.Mode())
			}

			// The manifest is re-read on every load.
			if err := os.WriteFile(filepath.Join(dir, spec.FileName), []byte(manifestFor("dev")), 0o644); err != nil {
				t.Fatal(err)
			}
			ext, err = m.Get("dev")
			if err != nil || ext.Manifest == nil {
				t.Fatalf("after adding a manifest: %+v, %v", ext, err)
			}

			if _, err := m.Install(context.Background(), src, InstallOptions{Pin: "v1"}); err == nil {
				t.Error("pin on a local install was accepted")
			}
			if err := m.Remove("dev"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, "redmine-cli-dev")); err != nil {
				t.Fatalf("remove deleted the linked directory: %v", err)
			}
			if _, err := m.Get("dev"); !errors.Is(err, ErrNotInstalled) {
				t.Fatalf("Get after remove: %v", err)
			}
		})
	}
}

func TestInstallLocalRejectsInvalidManifest(t *testing.T) {
	m, _, _ := testManager(t)
	dir := filepath.Join(t.TempDir(), "redmine-cli-dev")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "redmine-cli-dev"), []byte("#!/bin/sh\n"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, spec.FileName), []byte(manifestFor("other")), 0o644)
	_, err := m.Install(context.Background(), Source{Name: "dev", LocalDir: dir}, InstallOptions{})
	var invalid *InvalidManifestError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want InvalidManifestError", err)
	}
	var ve *spec.ValidationError
	if !errors.As(err, &ve) || len(ve.Details()) == 0 {
		t.Fatalf("err does not unwrap to the validation error: %v", err)
	}
}

func TestListAndBrokenExtensions(t *testing.T) {
	m, fg, git := testManager(t)
	asset := BinaryAssetName("bin", "linux", "amd64")
	fg.addRelease("octo/redmine-cli-bin", "v1.0.0", map[string][]byte{asset: []byte("x"), spec.FileName: []byte(manifestFor("bin"))})
	git.repos["fake://octo/redmine-cli-scr"] = map[string]string{"redmine-cli-scr*": "#!/bin/sh\n"}
	ctx := context.Background()
	if _, err := m.Install(ctx, Source{Owner: "octo", Repo: "redmine-cli-bin", Name: "bin"}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Install(ctx, Source{Owner: "octo", Repo: "redmine-cli-scr", Name: "scr"}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}

	// A git extension whose manifest broke after an upgrade.
	_ = os.WriteFile(filepath.Join(m.entryPath("scr"), spec.FileName), []byte("schema_version: 1\nname: scr\n"), 0o644)
	// A local link whose target is gone.
	if runtime.GOOS != "windows" {
		_ = os.Symlink(filepath.Join(t.TempDir(), "gone"), m.entryPath("gone"))
	}
	// A directory without executable.
	_ = os.MkdirAll(m.entryPath("noexe"), 0o755)
	// Entries that are not extensions.
	_ = os.MkdirAll(filepath.Join(m.Root, "random"), 0o755)
	_ = os.MkdirAll(filepath.Join(m.Root, "redmine-cli-Bad"), 0o755)

	exts, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*Extension{}
	var names []string
	for _, e := range exts {
		got[e.Name] = e
		names = append(names, e.Name)
	}
	want := []string{"bin", "gone", "noexe", "scr"}
	if runtime.GOOS == "windows" {
		want = []string{"bin", "noexe", "scr"}
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v, want %v", names, want)
	}
	if e := got["bin"]; e.Err != nil || e.Manifest == nil || e.Kind != KindBinary {
		t.Errorf("bin = %+v", e)
	}
	if e := got["scr"]; e.Err == nil || !strings.Contains(e.Err.Error(), "redmine-extension.yml") || e.Kind != KindGit {
		t.Errorf("scr err = %v", e.Err)
	}
	if e := got["noexe"]; e.Err == nil || !strings.Contains(e.Err.Error(), "no executable") {
		t.Errorf("noexe err = %v", e.Err)
	}
	if e, ok := got["gone"]; ok && (e.Err == nil || e.Kind != KindLocal) {
		t.Errorf("gone = %+v", e)
	}

	empty := &Manager{Root: filepath.Join(t.TempDir(), "missing")}
	if exts, err := empty.List(); err != nil || exts != nil {
		t.Fatalf("missing root: %v, %v", exts, err)
	}
}

func TestRemove(t *testing.T) {
	m, fg, git := testManager(t)
	ctx := context.Background()
	fg.addRelease("octo/redmine-cli-bin", "v1", map[string][]byte{BinaryAssetName("bin", "linux", "amd64"): []byte("x")})
	git.repos["fake://octo/redmine-cli-scr"] = map[string]string{"redmine-cli-scr*": "#!/bin/sh\n"}
	if _, err := m.Install(ctx, Source{Owner: "octo", Repo: "redmine-cli-bin", Name: "bin"}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Install(ctx, Source{Owner: "octo", Repo: "redmine-cli-scr", Name: "scr"}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bin", "scr"} {
		if err := m.Remove(name); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
		if _, err := os.Lstat(m.entryPath(name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still on disk", name)
		}
	}
	if _, err := os.Stat(m.installMetaPath("scr")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("git metadata not removed")
	}
	if err := m.Remove("bin"); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("second remove: %v", err)
	}
	if err := m.Remove("../etc"); err == nil {
		t.Fatal("invalid name accepted")
	}
}

func TestReadGitHead(t *testing.T) {
	sha := strings.Repeat("c", 40)
	tests := []struct {
		name  string
		files map[string]string
		want  string
		err   bool
	}{
		{name: "ref file", files: map[string]string{"HEAD": "ref: refs/heads/main\n", "refs/heads/main": sha + "\n"}, want: sha},
		{name: "packed refs", files: map[string]string{"HEAD": "ref: refs/heads/main\n", "packed-refs": "# pack-refs\n" + sha + " refs/heads/main\n"}, want: sha},
		{name: "detached", files: map[string]string{"HEAD": sha + "\n"}, want: sha},
		{name: "escaping ref", files: map[string]string{"HEAD": "ref: refs/../../x\n"}, err: true},
		{name: "missing ref", files: map[string]string{"HEAD": "ref: refs/heads/none\n"}, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(dir, ".git", filepath.FromSlash(name))
				_ = os.MkdirAll(filepath.Dir(path), 0o755)
				_ = os.WriteFile(path, []byte(content), 0o644)
			}
			got, err := readGitHead(dir)
			if tt.err {
				if err == nil {
					t.Fatalf("got %q, want an error", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestNames(t *testing.T) {
	if got := ExecutableName("pr", "windows"); got != "redmine-cli-pr.exe" {
		t.Errorf("ExecutableName windows = %q", got)
	}
	if got := ExecutableName("pr", "linux"); got != "redmine-cli-pr" {
		t.Errorf("ExecutableName linux = %q", got)
	}
	if got := BinaryAssetName("pr", "windows", "arm64"); got != "redmine-cli-pr-windows-arm64.exe" {
		t.Errorf("BinaryAssetName = %q", got)
	}
	if got := BinaryAssetName("pr", "darwin", "arm64"); got != "redmine-cli-pr-darwin-arm64" {
		t.Errorf("BinaryAssetName = %q", got)
	}
}

// TestReplaceCleanupIsBestEffort covers a swap whose new version is in
// place but whose old version cannot be deleted (on Windows, while the old
// executable runs): the install succeeds, and the next replace sweeps the
// leftover.
func TestReplaceCleanupIsBestEffort(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot delete from")
	}
	m, fg, _ := testManager(t)
	asset := BinaryAssetName("pr", "linux", "amd64")
	fg.addRelease("octo/redmine-cli-pr", "v1.0.0", map[string][]byte{asset: []byte("one")})
	src := Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}
	if _, err := m.Install(context.Background(), src, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	// A read-only subdirectory makes deleting the old entry fail.
	locked := filepath.Join(m.entryPath("pr"), "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	unlock := func() {
		entries, _ := os.ReadDir(m.Root)
		for _, e := range entries {
			_ = filepath.Walk(filepath.Join(m.Root, e.Name()), func(path string, info os.FileInfo, err error) error {
				if err == nil && info.IsDir() {
					_ = os.Chmod(path, 0o755)
				}
				return nil
			})
		}
	}
	t.Cleanup(unlock)

	fg.addRelease("octo/redmine-cli-pr", "v1.1.0", map[string][]byte{asset: []byte("two")})
	ext, err := m.Install(context.Background(), src, InstallOptions{Force: true})
	if err != nil {
		t.Fatalf("install reported a failure after the swap: %v", err)
	}
	if ext.Version() != "v1.1.0" {
		t.Fatalf("version = %q, want v1.1.0", ext.Version())
	}
	var old []string
	entries, _ := os.ReadDir(m.Root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".old-") {
			old = append(old, e.Name())
		}
	}
	if len(old) != 1 {
		t.Fatalf("leftovers = %v, want the undeletable old entry", old)
	}

	// An extension whose name extends this one keeps its leftovers.
	other := filepath.Join(m.Root, ".old-redmine-cli-pr-x-123")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	unlock()
	fg.addRelease("octo/redmine-cli-pr", "v1.2.0", map[string][]byte{asset: []byte("three")})
	if _, err := m.Install(context.Background(), src, InstallOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.Root, old[0])); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the leftover %s was not swept", old[0])
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("another extension's leftover was swept: %v", err)
	}
}

// TestInstallGitForceKeepsMetadataOnFailedSwap reinstalls a git extension
// with --force and makes the swap fail: the old clone stays, and so must
// its metadata, or it could no longer be upgraded.
func TestInstallGitForceKeepsMetadataOnFailedSwap(t *testing.T) {
	m, _, git := testManager(t)
	git.repos["fake://octo/redmine-cli-sh"] = map[string]string{"redmine-cli-sh*": "#!/bin/sh\n"}
	git.tags["fake://octo/redmine-cli-sh@v0.1.0"] = map[string]string{"redmine-cli-sh*": "#!/bin/sh\n"}
	src := Source{Owner: "octo", Repo: "redmine-cli-sh", Name: "sh"}
	if _, err := m.Install(context.Background(), src, InstallOptions{Pin: "v0.1.0"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(m.installMetaPath("sh"))
	if err != nil {
		t.Fatal(err)
	}

	final := m.entryPath("sh")
	m.renameFunc = func(from, to string) error {
		// Only moving the new clone in fails; restoring the old one works.
		if to == final && strings.HasPrefix(filepath.Base(from), ".tmp-") {
			return errors.New("swap failed")
		}
		return os.Rename(from, to)
	}
	if _, err := m.Install(context.Background(), src, InstallOptions{Force: true}); err == nil || !strings.Contains(err.Error(), "swap failed") {
		t.Fatalf("err = %v, want the swap failure", err)
	}
	m.renameFunc = nil

	after, err := os.ReadFile(m.installMetaPath("sh"))
	if err != nil {
		t.Fatalf("metadata gone after a failed reinstall: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("metadata changed:\n%s\nwant\n%s", after, before)
	}
	ext, err := m.Get("sh")
	if err != nil || ext.Kind != KindGit || !ext.Pinned() || ext.Version() != "v0.1.0" {
		t.Fatalf("ext = %+v, %v; want the old pinned clone", ext, err)
	}
	assertNoLeftovers(t, m)
}

// TestCloneInstallFileIsIgnored installs a git extension whose repository
// ships an install.yml claiming another repository, a release tag and a
// pin: none of it may be trusted. The extension stays a git extension of
// the repository it was cloned from, unpinned and upgradable.
func TestCloneInstallFileIsIgnored(t *testing.T) {
	m, _, git := testManager(t)
	forged := "owner: evil\nrepo: redmine-cli-other\ntag: v9.9.9\npinned: true\nkind: binary\nchecksum_verified: true\n"
	git.repos["fake://octo/redmine-cli-sh"] = map[string]string{"redmine-cli-sh*": "#!/bin/sh\n", "install.yml": forged}
	src := Source{Owner: "octo", Repo: "redmine-cli-sh", Name: "sh"}
	if _, err := m.Install(context.Background(), src, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	check := func(when string) {
		t.Helper()
		ext, err := m.Get("sh")
		if err != nil {
			t.Fatal(err)
		}
		if ext.Kind != KindGit || ext.Repo() != "octo/redmine-cli-sh" || ext.Pinned() || ext.Version() != shortRevision(ext.Revision) || ext.Install.ChecksumVerified {
			t.Fatalf("%s: ext = %+v, install %+v; the clone's install.yml was trusted", when, ext, ext.Install)
		}
	}
	check("after install")

	res, err := m.Upgrade(context.Background(), "sh", UpgradeOptions{})
	if err != nil || res.Status != StatusUpgraded {
		t.Fatalf("upgrade = %+v, %v; want an upgraded git extension", res, err)
	}
	check("after upgrade")

	// Even without any metadata, the file is not read.
	if err := os.Remove(m.installMetaPath("sh")); err != nil {
		t.Fatal(err)
	}
	ext, err := m.Get("sh")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Kind != KindGit || ext.Install != nil || ext.Repo() != "" || ext.Pinned() {
		t.Fatalf("without metadata: ext = %+v", ext)
	}
}

// TestInstallMetadataOutsideExtensionDir checks that no kind of install
// writes metadata into the extension directory.
func TestInstallMetadataOutsideExtensionDir(t *testing.T) {
	m, fg, git := testManager(t)
	fg.addRelease("octo/redmine-cli-bin", "v1.0.0", map[string][]byte{BinaryAssetName("bin", "linux", "amd64"): []byte("x")})
	git.repos["fake://octo/redmine-cli-scr"] = map[string]string{"redmine-cli-scr*": "#!/bin/sh\n"}
	ctx := context.Background()
	for _, src := range []Source{
		{Owner: "octo", Repo: "redmine-cli-bin", Name: "bin"},
		{Owner: "octo", Repo: "redmine-cli-scr", Name: "scr"},
	} {
		ext, err := m.Install(ctx, src, InstallOptions{})
		if err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(ext.Dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != filepath.Base(ext.Path) && e.Name() != ".git" {
				t.Errorf("%s: unexpected %s in the extension directory", src.Name, e.Name())
			}
		}
		meta, err := readInstallInfo(m.installMetaPath(src.Name))
		if err != nil || meta.Owner != "octo" || meta.Kind != ext.Kind {
			t.Errorf("%s: metadata = %+v, %v", src.Name, meta, err)
		}
	}
}

// TestInstallBinaryWarnings covers the install warnings and the recorded
// checksum verification.
func TestInstallBinaryWarnings(t *testing.T) {
	bin := []byte("bin")
	manifest := []byte(manifestFor("pr"))
	asset := BinaryAssetName("pr", "linux", "amd64")
	tests := []struct {
		name         string
		assets       map[string][]byte
		wantWarnings []string
		wantVerified bool
	}{
		{name: "verified with manifest", assets: map[string][]byte{
			asset: bin, spec.FileName: manifest, "checksums.txt": checksumsFor(map[string][]byte{asset: bin, spec.FileName: manifest}),
		}, wantVerified: true},
		{name: "no checksums", assets: map[string][]byte{asset: bin, spec.FileName: manifest}, wantWarnings: []string{
			"release v1.0.0 of octo/redmine-cli-pr has no checksums file: " + asset + " and redmine-extension.yml were installed without checksum verification",
		}},
		{name: "manifest not in checksums", assets: map[string][]byte{
			asset: bin, spec.FileName: manifest, "checksums.txt": checksumsFor(map[string][]byte{asset: bin}),
		}, wantWarnings: []string{
			"the checksums file of release v1.0.0 of octo/redmine-cli-pr does not list redmine-extension.yml: it was installed without checksum verification",
		}},
		{name: "no manifest", assets: map[string][]byte{
			asset: bin, "checksums.txt": checksumsFor(map[string][]byte{asset: bin}),
		}, wantVerified: true, wantWarnings: []string{
			"release v1.0.0 of octo/redmine-cli-pr has no redmine-extension.yml asset: installed as a passthrough extension, without declarative commands or MCP tools",
		}},
		{name: "nothing verified, no manifest", assets: map[string][]byte{asset: bin}, wantWarnings: []string{
			"release v1.0.0 of octo/redmine-cli-pr has no checksums file: " + asset + " was installed without checksum verification",
			"release v1.0.0 of octo/redmine-cli-pr has no redmine-extension.yml asset: installed as a passthrough extension, without declarative commands or MCP tools",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fg, _ := testManager(t)
			fg.addRelease("octo/redmine-cli-pr", "v1.0.0", tt.assets)
			var warnings []string
			ext, err := m.Install(context.Background(), Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}, InstallOptions{
				Warn: func(w string) { warnings = append(warnings, w) },
			})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(warnings, "\n") != strings.Join(tt.wantWarnings, "\n") {
				t.Errorf("warnings:\n%s\nwant:\n%s", strings.Join(warnings, "\n"), strings.Join(tt.wantWarnings, "\n"))
			}
			if ext.Install.ChecksumVerified != tt.wantVerified {
				t.Errorf("checksum verified = %v, want %v", ext.Install.ChecksumVerified, tt.wantVerified)
			}
		})
	}
}

// TestLoadSkipsUnknownManifestFields: an installed extension whose manifest
// has a field this redmine-cli does not know still loads, with a warning,
// while installing it is refused.
func TestLoadSkipsUnknownManifestFields(t *testing.T) {
	m, _, _ := testManager(t)
	dir := filepath.Join(t.TempDir(), "redmine-cli-dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redmine-cli-dev"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, spec.FileName), []byte(manifestFor("dev")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Install(context.Background(), Source{Name: "dev", LocalDir: dir}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, spec.FileName), []byte(manifestFor("dev")+"    future_field: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ext, err := m.Get("dev")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Err != nil || ext.Manifest == nil || ext.Manifest.Command("list") == nil {
		t.Fatalf("ext = %+v (err %v), want the manifest loaded", ext, ext.Err)
	}
	if len(ext.Warnings) != 1 || !strings.Contains(ext.Warnings[0].Error(), "future_field") {
		t.Fatalf("warnings = %v, want one for future_field", ext.Warnings)
	}

	var invalid *InvalidManifestError
	if _, err := m.Install(context.Background(), Source{Name: "dev", LocalDir: dir}, InstallOptions{Force: true}); !errors.As(err, &invalid) {
		t.Fatalf("install err = %v, want the strict manifest check to fail", err)
	}
}
