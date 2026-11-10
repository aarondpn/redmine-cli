package extension

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

func TestUpgradeBinary(t *testing.T) {
	asset := BinaryAssetName("pr", "linux", "amd64")
	tests := []struct {
		name       string
		newTag     string
		newAssets  map[string][]byte
		opts       UpgradeOptions
		pin        string
		wantStatus UpgradeStatus
		wantTo     string
		wantData   string
		wantErr    string
	}{
		{name: "up to date", wantStatus: StatusUpToDate, wantTo: "v1.0.0", wantData: "one"},
		{name: "newer release", newTag: "v1.1.0", newAssets: map[string][]byte{asset: []byte("two")}, wantStatus: StatusUpgraded, wantTo: "v1.1.0", wantData: "two"},
		{name: "dry run", newTag: "v1.1.0", newAssets: map[string][]byte{asset: []byte("two")}, opts: UpgradeOptions{DryRun: true}, wantStatus: StatusAvailable, wantTo: "v1.1.0", wantData: "one"},
		{name: "force reinstalls", opts: UpgradeOptions{Force: true}, wantStatus: StatusUpgraded, wantTo: "v1.0.0", wantData: "one"},
		{name: "pinned is skipped", pin: "v1.0.0", newTag: "v1.1.0", newAssets: map[string][]byte{asset: []byte("two")}, wantStatus: StatusSkipped, wantData: "one"},
		{name: "new release without binary", newTag: "v1.1.0", newAssets: map[string][]byte{"other": []byte("x")}, wantErr: "no binary for linux/amd64"},
		{name: "new release with broken manifest keeps the old one", newTag: "v1.1.0", newAssets: map[string][]byte{asset: []byte("two"), spec.FileName: []byte("name: pr\n")}, wantErr: "invalid redmine-extension.yml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fg, _ := testManager(t)
			fg.addRelease("octo/redmine-cli-pr", "v1.0.0", map[string][]byte{asset: []byte("one")})
			src := Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}
			if _, err := m.Install(context.Background(), src, InstallOptions{Pin: tt.pin}); err != nil {
				t.Fatal(err)
			}
			if tt.newTag != "" {
				fg.addRelease("octo/redmine-cli-pr", tt.newTag, tt.newAssets)
			}
			res, err := m.Upgrade(context.Background(), "pr", tt.opts)
			ext, getErr := m.Get("pr")
			if getErr != nil {
				t.Fatal(getErr)
			}
			data, _ := os.ReadFile(ext.Path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if string(data) != "one" || ext.Version() != "v1.0.0" {
					t.Fatalf("failed upgrade changed the install: %q %s", data, ext.Version())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != tt.wantStatus || res.To != tt.wantTo || res.From != "v1.0.0" {
				t.Fatalf("result = %+v", res)
			}
			if string(data) != tt.wantData {
				t.Fatalf("binary = %q, want %q", data, tt.wantData)
			}
			if tt.wantStatus == StatusSkipped && !strings.Contains(res.Note, "pinned") {
				t.Errorf("note = %q", res.Note)
			}
		})
	}
}

func TestUpgradeGit(t *testing.T) {
	tests := []struct {
		name       string
		opts       UpgradeOptions
		upstream   string
		wantStatus UpgradeStatus
		wantHead   string
		wantPulls  int
		wantResets int
	}{
		{name: "pull", upstream: strings.Repeat("b", 40), wantStatus: StatusUpgraded, wantHead: strings.Repeat("b", 40), wantPulls: 1},
		{name: "already current", upstream: initialRevision, wantStatus: StatusUpToDate, wantHead: initialRevision, wantPulls: 1},
		{name: "dry run", opts: UpgradeOptions{DryRun: true}, upstream: strings.Repeat("b", 40), wantStatus: StatusAvailable, wantHead: initialRevision},
		{name: "dry run current", opts: UpgradeOptions{DryRun: true}, upstream: initialRevision, wantStatus: StatusUpToDate, wantHead: initialRevision},
		{name: "force resets", opts: UpgradeOptions{Force: true}, upstream: strings.Repeat("b", 40), wantStatus: StatusUpgraded, wantHead: strings.Repeat("b", 40), wantResets: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _, git := testManager(t)
			git.repos["fake://octo/redmine-cli-scr"] = map[string]string{"redmine-cli-scr*": "#!/bin/sh\n"}
			if _, err := m.Install(context.Background(), Source{Owner: "octo", Repo: "redmine-cli-scr", Name: "scr"}, InstallOptions{}); err != nil {
				t.Fatal(err)
			}
			git.upstream = tt.upstream
			res, err := m.Upgrade(context.Background(), "scr", tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != tt.wantStatus || res.From != initialRevision[:7] {
				t.Fatalf("result = %+v", res)
			}
			head, _ := readGitHead(m.entryPath("scr"))
			if head != tt.wantHead {
				t.Errorf("HEAD = %s, want %s", head, tt.wantHead)
			}
			if git.pulls != tt.wantPulls || git.resets != tt.wantResets {
				t.Errorf("pulls=%d resets=%d", git.pulls, git.resets)
			}
		})
	}
}

func TestUpgradeGitReportsBrokenManifest(t *testing.T) {
	m, _, git := testManager(t)
	git.repos["fake://octo/redmine-cli-scr"] = map[string]string{"redmine-cli-scr*": "#!/bin/sh\n"}
	if _, err := m.Install(context.Background(), Source{Owner: "octo", Repo: "redmine-cli-scr", Name: "scr"}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(m.entryPath("scr"), spec.FileName), []byte("schema_version: 1\n"), 0o644)
	res, err := m.Upgrade(context.Background(), "scr", UpgradeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Note, "invalid") {
		t.Fatalf("note = %q", res.Note)
	}
}

func TestUpgradeAll(t *testing.T) {
	m, fg, git := testManager(t)
	ctx := context.Background()
	asset := BinaryAssetName("bin", "linux", "amd64")
	fg.addRelease("octo/redmine-cli-bin", "v1", map[string][]byte{asset: []byte("x")})
	fg.addRelease("octo/redmine-cli-gone", "v1", map[string][]byte{BinaryAssetName("gone", "linux", "amd64"): []byte("x")})
	git.repos["fake://octo/redmine-cli-scr"] = map[string]string{"redmine-cli-scr*": "#!/bin/sh\n"}
	for _, src := range []Source{
		{Owner: "octo", Repo: "redmine-cli-bin", Name: "bin"},
		{Owner: "octo", Repo: "redmine-cli-gone", Name: "gone"},
		{Owner: "octo", Repo: "redmine-cli-scr", Name: "scr"},
	} {
		if _, err := m.Install(ctx, src, InstallOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	local := filepath.Join(t.TempDir(), "redmine-cli-dev")
	_ = os.MkdirAll(local, 0o755)
	_ = os.WriteFile(filepath.Join(local, "redmine-cli-dev"), []byte("#!/bin/sh\n"), 0o755)
	if _, err := m.Install(ctx, Source{Name: "dev", LocalDir: local}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	fg.addRelease("octo/redmine-cli-bin", "v2", map[string][]byte{asset: []byte("y")})
	// The repository of "gone" lost its releases.
	fg.mu.Lock()
	delete(fg.releases, "octo/redmine-cli-gone")
	fg.mu.Unlock()

	results, err := m.UpgradeAll(ctx, UpgradeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]UpgradeStatus{}
	for _, r := range results {
		got[r.Name] = r.Status
	}
	want := map[string]UpgradeStatus{"bin": StatusUpgraded, "dev": StatusSkipped, "gone": StatusFailed, "scr": StatusUpgraded}
	for name, status := range want {
		if got[name] != status {
			t.Errorf("%s: status %q, want %q (results %+v)", name, got[name], status, results)
		}
	}
}

func TestCheckForUpdate(t *testing.T) {
	m, fg, git := testManager(t)
	ctx := context.Background()
	asset := BinaryAssetName("bin", "linux", "amd64")
	fg.addRelease("octo/redmine-cli-bin", "v1.0.0", map[string][]byte{asset: []byte("x")})
	git.repos["fake://octo/redmine-cli-scr"] = map[string]string{"redmine-cli-scr*": "#!/bin/sh\n"}
	bin, err := m.Install(ctx, Source{Owner: "octo", Repo: "redmine-cli-bin", Name: "bin"}, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	scr, err := m.Install(ctx, Source{Owner: "octo", Repo: "redmine-cli-scr", Name: "scr"}, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Up to date: no notice, but the result is cached.
	n, err := m.CheckForUpdate(ctx, bin)
	if err != nil || n != nil {
		t.Fatalf("up to date: %+v, %v", n, err)
	}
	before := fg.requestCount("/repos/octo/redmine-cli-bin/releases")
	fg.addRelease("octo/redmine-cli-bin", "v1.1.0", map[string][]byte{asset: []byte("y")})
	if n, _ := m.CheckForUpdate(ctx, bin); n != nil {
		t.Fatalf("cached negative result ignored: %+v", n)
	}
	if after := fg.requestCount("/repos/octo/redmine-cli-bin/releases"); after != before {
		t.Fatalf("cached check hit the API (%d -> %d)", before, after)
	}

	// A day later the new release is found.
	later := m.now().Add(NoticeCacheAge + time.Minute)
	m.Now = func() time.Time { return later }
	n, err = m.CheckForUpdate(ctx, bin)
	if err != nil || n == nil || n.Current != "v1.0.0" || n.Latest != "v1.1.0" {
		t.Fatalf("new release: %+v, %v", n, err)
	}

	// Git: compare the head commit of the default branch.
	fg.commits["octo/redmine-cli-scr"] = strings.Repeat("d", 40)
	n, err = m.CheckForUpdate(ctx, scr)
	if err != nil || n == nil || n.Current != initialRevision[:7] || n.Latest != "ddddddd" {
		t.Fatalf("git: %+v, %v", n, err)
	}

	// An upgrade invalidates the cache.
	if _, err := m.Upgrade(ctx, "bin", UpgradeOptions{}); err != nil {
		t.Fatal(err)
	}
	bin, _ = m.Get("bin")
	if n, err := m.CheckForUpdate(ctx, bin); err != nil || n != nil {
		t.Fatalf("after upgrade: %+v, %v", n, err)
	}

	// A tampered cache entry is ignored.
	_ = os.WriteFile(m.noticePath("bin"), []byte(`{"checked_at":"2999-01-01T00:00:00Z","current":"v1.1.0","latest":"v9"}`), 0o600)
	if c := m.readNotice("bin"); c != nil {
		t.Fatalf("future cache entry accepted: %+v", c)
	}
	_ = os.WriteFile(m.noticePath("bin"), []byte(`{"checked_at":"2026-11-01T00:00:00Z","current":"v1.1.0","latest":"\u001b[31mv9"}`), 0o600)
	if c := m.readNotice("bin"); c != nil {
		t.Fatalf("unsafe cache entry accepted: %+v", c)
	}
}

func TestNoticeEligible(t *testing.T) {
	info := &InstallInfo{Owner: "o", Repo: "redmine-cli-x", Tag: "v1"}
	tests := []struct {
		name string
		ext  *Extension
		want bool
	}{
		{name: "binary", ext: &Extension{Kind: KindBinary, Install: info}, want: true},
		{name: "git", ext: &Extension{Kind: KindGit, Install: &InstallInfo{Owner: "o", Repo: "r"}, Revision: "abc"}, want: true},
		{name: "git without revision", ext: &Extension{Kind: KindGit, Install: &InstallInfo{Owner: "o", Repo: "r"}}},
		{name: "pinned", ext: &Extension{Kind: KindBinary, Install: &InstallInfo{Owner: "o", Repo: "r", Tag: "v1", Pinned: true}}},
		{name: "local", ext: &Extension{Kind: KindLocal}},
		{name: "nil", ext: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NoticeEligible(tt.ext); got != tt.want {
				t.Fatalf("NoticeEligible = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSearch(t *testing.T) {
	m, fg, _ := testManager(t)
	fg.addRelease("octo/redmine-cli-pr", "v1", map[string][]byte{BinaryAssetName("pr", "linux", "amd64"): []byte("x")})
	if _, err := m.Install(context.Background(), Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}, InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	fg.search = []map[string]any{
		{"full_name": "Octo/redmine-cli-pr", "name": "redmine-cli-pr", "description": "PRs", "stargazers_count": 5, "html_url": "https://github.com/octo/redmine-cli-pr"},
		{"full_name": "other/redmine-cli-deploy", "name": "redmine-cli-deploy", "description": "Deploys", "stargazers_count": 2, "html_url": "https://github.com/other/redmine-cli-deploy"},
		{"full_name": "other/unrelated", "name": "unrelated"},
		{"full_name": "other/redmine-cli-Bad", "name": "redmine-cli-Bad"},
	}
	results, err := m.Search(context.Background(), "  deploy ", 10)
	if err != nil {
		t.Fatal(err)
	}
	if fg.lastSearch != "deploy topic:"+Topic {
		t.Errorf("q = %q", fg.lastSearch)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Name != "pr" || !results[0].Installed || results[0].Stars != 5 {
		t.Errorf("first = %+v", results[0])
	}
	if results[1].Name != "deploy" || results[1].Installed {
		t.Errorf("second = %+v", results[1])
	}

	if _, err := m.Search(context.Background(), "", 0); err != nil {
		t.Fatal(err)
	}
	if fg.lastSearch != "topic:"+Topic {
		t.Errorf("empty query q = %q", fg.lastSearch)
	}
}

// TestUpgradeKeepsChecksumVerification refuses an upgrade from a verified
// release to one that cannot be verified, unless forced.
func TestUpgradeKeepsChecksumVerification(t *testing.T) {
	asset := BinaryAssetName("pr", "linux", "amd64")
	one, two := []byte("one"), []byte("two")
	tests := []struct {
		name         string
		verified     bool
		newAssets    map[string][]byte
		force        bool
		wantErr      string
		wantWarnings int
		wantVerified bool
	}{
		{name: "verified to unverified fails", verified: true, newAssets: map[string][]byte{asset: two}, wantErr: "has no checksums file, but the installed version was checksum-verified; use --force"},
		{name: "manifest no longer listed fails", verified: true, newAssets: map[string][]byte{
			asset: two, spec.FileName: []byte(manifestFor("pr")), "checksums.txt": checksumsFor(map[string][]byte{asset: two}),
		}, wantErr: "does not list redmine-extension.yml"},
		{name: "forced", verified: true, newAssets: map[string][]byte{asset: two}, force: true, wantWarnings: 2},
		{name: "verified to verified", verified: true, newAssets: map[string][]byte{asset: two, "checksums.txt": checksumsFor(map[string][]byte{asset: two})}, wantWarnings: 1, wantVerified: true},
		{name: "unverified to unverified", newAssets: map[string][]byte{asset: two}, wantWarnings: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fg, _ := testManager(t)
			first := map[string][]byte{asset: one}
			if tt.verified {
				first["checksums.txt"] = checksumsFor(map[string][]byte{asset: one})
			}
			fg.addRelease("octo/redmine-cli-pr", "v1.0.0", first)
			if _, err := m.Install(context.Background(), Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}, InstallOptions{}); err != nil {
				t.Fatal(err)
			}
			fg.addRelease("octo/redmine-cli-pr", "v1.1.0", tt.newAssets)
			res, err := m.Upgrade(context.Background(), "pr", UpgradeOptions{Force: tt.force})
			ext, getErr := m.Get("pr")
			if getErr != nil {
				t.Fatal(getErr)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if ext.Version() != "v1.0.0" || !ext.Install.ChecksumVerified {
					t.Fatalf("a refused upgrade changed the install: %+v", ext.Install)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != StatusUpgraded || ext.Version() != "v1.1.0" || ext.Install.ChecksumVerified != tt.wantVerified {
				t.Fatalf("result = %+v, install = %+v", res, ext.Install)
			}
			if len(res.Warnings) != tt.wantWarnings {
				t.Errorf("warnings = %q, want %d", res.Warnings, tt.wantWarnings)
			}
		})
	}
}
