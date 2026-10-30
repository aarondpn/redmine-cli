package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const scriptFixtureTag = "v2.14.1"

// scriptTools are the external commands install.sh needs besides curl and
// gh, which are stubbed. Missing optional ones (e.g. sha256sum on macOS) are
// skipped.
var scriptTools = []string{
	"uname", "mktemp", "rm", "grep", "cut", "tar", "gzip", "chmod", "mkdir",
	"mv", "cp", "cat", "sha256sum", "shasum",
}

// curlStub serves release assets from $FIXTURE_DIR for $FIXTURE_TAG and a
// releases/latest API response naming $LATEST_TAG. Every URL is logged.
const curlStub = `out=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -*) shift ;;
    *)  url="$1"; shift ;;
  esac
done
printf '%s\n' "$url" >> "$STUB_LOG"
case "$url" in
  */releases/latest) printf '{"tag_name": "%s"}\n' "$LATEST_TAG"; exit 0 ;;
  */releases/download/"$FIXTURE_TAG"/*) ;;
  *) exit 22 ;;
esac
src="$FIXTURE_DIR/${url##*/}"
[ -f "$src" ] || exit 22
if [ -n "$out" ]; then cp "$src" "$out"; else cat "$src"; fi
`

// ghStub logs its arguments and exits with $GH_AUTH_EXIT for `auth status`
// and $GH_VERIFY_EXIT for `attestation verify`. `attestation verify --help`
// lists the flags in $GH_VERIFY_FLAGS (default: both flags install.sh uses).
const ghStub = `printf 'gh %s\n' "$*" >> "$STUB_LOG"
if [ "$1 $2 $3" = "attestation verify --help" ]; then
  printf 'Verify the integrity and provenance of an artifact.\n\n'
  printf 'Use the --signer-workflow and --source-ref flags to pin the identity.\n\nFLAGS\n'
  for f in ${GH_VERIFY_FLAGS-signer-workflow source-ref}; do
    printf '      --%s string   description\n' "$f"
  done
  exit 0
fi
case "$1 $2" in
  "attestation --help") exit 0 ;;
  "auth status") exit "${GH_AUTH_EXIT:-0}" ;;
  "attestation verify") exit "${GH_VERIFY_EXIT:-0}" ;;
esac
exit 1
`

type scriptEnv struct {
	dir        string
	installDir string
	log        string
	archive    string
}

// newScriptEnv prepares a PATH with only the tools install.sh needs, a stub
// curl serving a fake release, and (if withGh) a stub gh.
func newScriptEnv(t *testing.T, withGh bool) *scriptEnv {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("install.sh supports Linux and macOS only")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("install.sh supports amd64 and arm64 only")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}

	dir := t.TempDir()
	env := &scriptEnv{
		dir:        dir,
		installDir: filepath.Join(dir, "install"),
		log:        filepath.Join(dir, "stub.log"),
		archive:    fmt.Sprintf("redmine-cli-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH),
	}
	binDir := filepath.Join(dir, "path")
	fixtureDir := filepath.Join(dir, "fixture")
	for _, d := range []string{binDir, fixtureDir, filepath.Join(dir, "tmp"), filepath.Join(dir, "home")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range append([]string{"bash"}, scriptTools...) {
		p, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		if err := os.Symlink(p, filepath.Join(binDir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	writeStub(t, filepath.Join(binDir, "curl"), bash, curlStub)
	if withGh {
		writeStub(t, filepath.Join(binDir, "gh"), bash, ghStub)
	}

	archive := buildTarGz(t, "redmine", []byte("#!/bin/sh\necho fake redmine\n"))
	if err := os.WriteFile(filepath.Join(fixtureDir, env.archive), archive, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), env.archive)
	checksumsName := "redmine-cli_" + strings.TrimPrefix(scriptFixtureTag, "v") + "_checksums.txt"
	if err := os.WriteFile(filepath.Join(fixtureDir, checksumsName), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}
	return env
}

func writeStub(t *testing.T, path, bash, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!"+bash+"\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func buildTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// run executes install.sh with a minimal environment plus extra.
func (e *scriptEnv) run(t *testing.T, extra map[string]string) (string, error) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(e.dir, "path", "bash"), script)
	cmd.Env = []string{
		"PATH=" + filepath.Join(e.dir, "path"),
		"HOME=" + filepath.Join(e.dir, "home"),
		"TMPDIR=" + filepath.Join(e.dir, "tmp"),
		"REDMINE_INSTALL_DIR=" + e.installDir,
		"STUB_LOG=" + e.log,
		"FIXTURE_DIR=" + filepath.Join(e.dir, "fixture"),
		"FIXTURE_TAG=" + scriptFixtureTag,
		"LATEST_TAG=" + scriptFixtureTag,
	}
	for k, v := range extra {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// corruptChecksums replaces the fixture's checksum for the archive.
func (e *scriptEnv) corruptChecksums(t *testing.T) {
	t.Helper()
	name := "redmine-cli_" + strings.TrimPrefix(scriptFixtureTag, "v") + "_checksums.txt"
	sums := strings.Repeat("0", 64) + "  " + e.archive + "\n"
	if err := os.WriteFile(filepath.Join(e.dir, "fixture", name), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e *scriptEnv) installed() bool {
	_, err := os.Stat(filepath.Join(e.installDir, "redmine"))
	return err == nil
}

func (e *scriptEnv) stubLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallScript(t *testing.T) {
	// gh verifies the downloaded archive (in a mktemp directory) against the
	// release workflow identity for exactly the installed tag.
	verifyArgs := fmt.Sprintf("/redmine-cli-%s-%s.tar.gz --repo aarondpn/redmine-cli"+
		" --signer-workflow aarondpn/redmine-cli/.github/workflows/release.yml"+
		" --source-ref refs/tags/%s\n", runtime.GOOS, runtime.GOARCH, scriptFixtureTag)

	tests := []struct {
		name          string
		withGh        bool
		corruptSums   bool // the release checksums do not match the archive
		env           map[string]string
		wantErr       bool
		wantInstalled bool
		wantOutput    []string
		wantLog       []string
		wantLogNot    []string
	}{
		{
			name:          "latest release with verified attestation",
			withGh:        true,
			wantInstalled: true,
			wantOutput:    []string{"Latest version: " + scriptFixtureTag, "Attestation verified.", "is not in your PATH", "/redmine auth login' to connect to your Redmine server", "https://redmine-cli.dev/getting-started/configuration/"},
			wantLog:       []string{"/releases/latest", "/releases/download/" + scriptFixtureTag + "/", "gh attestation verify ", verifyArgs},
		},
		{
			name:          "pinned version without v prefix",
			withGh:        true,
			env:           map[string]string{"REDMINE_CLI_VERSION": strings.TrimPrefix(scriptFixtureTag, "v")},
			wantInstalled: true,
			wantOutput:    []string{"Requested version: " + scriptFixtureTag},
			wantLog:       []string{"/releases/download/" + scriptFixtureTag + "/"},
			wantLogNot:    []string{"/releases/latest"},
		},
		{
			// The official Redmine Docker image sets REDMINE_VERSION to the
			// Redmine release; it must not pin a redmine-cli tag.
			name:          "REDMINE_VERSION of the Redmine image is ignored",
			withGh:        true,
			env:           map[string]string{"REDMINE_VERSION": "7.0.2"},
			wantInstalled: true,
			wantOutput:    []string{"Latest version: " + scriptFixtureTag},
			wantLog:       []string{"/releases/latest"},
			wantLogNot:    []string{"v7.0.2"},
		},
		{
			name:       "failed attestation refuses to install",
			withGh:     true,
			env:        map[string]string{"GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"Attestation verification failed"},
		},
		{
			name:       "failed attestation names the escape hatch with a caveat",
			withGh:     true,
			env:        map[string]string{"GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"REDMINE_CLI_SKIP_ATTESTATION=1", "If the attestation itself is invalid, do not\ninstall"},
		},
		{
			// Pinning a release from before attestations existed fails
			// verification; the error must say how to install it anyway.
			name:       "failed attestation mentions releases without attestations",
			withGh:     true,
			env:        map[string]string{"GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"Releases older than v2.11.2 have no attestations"},
		},
		{
			// gh 2.49 has `gh attestation` but not --source-ref; verifying
			// with it would fail for a valid release.
			name:          "gh without --source-ref falls back to manual verification",
			withGh:        true,
			env:           map[string]string{"GH_VERIFY_FLAGS": "signer-workflow", "GH_VERIFY_EXIT": "1"},
			wantInstalled: true,
			wantOutput:    []string{"lacks --signer-workflow or --source-ref", "gh attestation verify "},
			wantLog:       []string{"gh attestation verify --help"},
			wantLogNot:    []string{"tar.gz --repo"},
		},
		{
			name:          "gh without --signer-workflow falls back to manual verification",
			withGh:        true,
			env:           map[string]string{"GH_VERIFY_FLAGS": "source-ref", "GH_VERIFY_EXIT": "1"},
			wantInstalled: true,
			wantOutput:    []string{"lacks --signer-workflow or --source-ref"},
			wantLogNot:    []string{"tar.gz --repo"},
		},
		{
			// Flag names mentioned only in the help prose do not count.
			name:          "gh listing neither flag falls back to manual verification",
			withGh:        true,
			env:           map[string]string{"GH_VERIFY_FLAGS": "", "GH_VERIFY_EXIT": "1"},
			wantInstalled: true,
			wantOutput:    []string{"lacks --signer-workflow or --source-ref"},
			wantLogNot:    []string{"tar.gz --repo"},
		},
		{
			// Escape hatch for networks that block Sigstore or a gh logged
			// in to GitHub Enterprise Server only.
			name:          "REDMINE_CLI_SKIP_ATTESTATION skips gh with a warning",
			withGh:        true,
			env:           map[string]string{"REDMINE_CLI_SKIP_ATTESTATION": "1", "GH_VERIFY_EXIT": "1"},
			wantInstalled: true,
			wantOutput: []string{
				"REDMINE_CLI_SKIP_ATTESTATION is set; skipping attestation verification",
				"a tampered release would NOT be detected",
				"Checksum verified.",
				"gh attestation verify ",
			},
			wantLogNot: []string{"gh attestation verify", "gh auth status"},
		},
		{
			name:          "REDMINE_CLI_SKIP_ATTESTATION=true without gh",
			env:           map[string]string{"REDMINE_CLI_SKIP_ATTESTATION": "true"},
			wantInstalled: true,
			wantOutput:    []string{"REDMINE_CLI_SKIP_ATTESTATION is set"},
		},
		{
			name:       "REDMINE_CLI_SKIP_ATTESTATION=0 still verifies",
			withGh:     true,
			env:        map[string]string{"REDMINE_CLI_SKIP_ATTESTATION": "0", "GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"Attestation verification failed"},
		},
		{
			// Skipping only drops the attestation; a checksum mismatch
			// still refuses.
			name:        "REDMINE_CLI_SKIP_ATTESTATION keeps the checksum check",
			withGh:      true,
			env:         map[string]string{"REDMINE_CLI_SKIP_ATTESTATION": "1"},
			corruptSums: true,
			wantErr:     true,
			wantOutput:  []string{"Checksum mismatch"},
		},
		{
			name:          "no gh prints manual verification command",
			wantInstalled: true,
			wantOutput:    []string{"GitHub CLI (gh) not found", "gh attestation verify "},
		},
		{
			name:          "logged-out gh prints manual verification command",
			withGh:        true,
			env:           map[string]string{"GH_AUTH_EXIT": "1"},
			wantInstalled: true,
			wantOutput:    []string{"not logged in"},
			wantLogNot:    []string{"gh attestation verify"},
		},
		{
			// A token makes gh usable without `gh auth login`.
			name:       "token skips the login check",
			withGh:     true,
			env:        map[string]string{"GH_AUTH_EXIT": "1", "GH_TOKEN": "x", "GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"Attestation verification failed"},
		},
		{
			name:       "invalid pinned tag",
			env:        map[string]string{"REDMINE_CLI_VERSION": "v1;touch pwned"},
			wantErr:    true,
			wantOutput: []string{"Invalid release tag"},
			wantLogNot: []string{"/releases/download/"},
		},
		{
			// Each line is a valid tag on its own; the whole value is not.
			name:       "multi-line pinned tag",
			env:        map[string]string{"REDMINE_CLI_VERSION": scriptFixtureTag + "\nv0.0.1"},
			wantErr:    true,
			wantOutput: []string{"Invalid release tag"},
			wantLogNot: []string{"/releases/download/"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newScriptEnv(t, tt.withGh)
			if tt.corruptSums {
				env.corruptChecksums(t)
			}
			out, err := env.run(t, tt.env)
			if tt.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr %v\noutput:\n%s", err, tt.wantErr, out)
			}
			if got := env.installed(); got != tt.wantInstalled {
				t.Errorf("installed = %v, want %v\noutput:\n%s", got, tt.wantInstalled, out)
			}
			for _, s := range tt.wantOutput {
				if !strings.Contains(out, s) {
					t.Errorf("expected output to contain %q, got:\n%s", s, out)
				}
			}
			log := env.stubLog(t)
			for _, s := range tt.wantLog {
				if !strings.Contains(log, s) {
					t.Errorf("expected stub log to contain %q, got:\n%s", s, log)
				}
			}
			for _, s := range tt.wantLogNot {
				if strings.Contains(log, s) {
					t.Errorf("expected stub log not to contain %q, got:\n%s", s, log)
				}
			}
		})
	}
}
