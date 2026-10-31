package install

import (
	"archive/zip"
	"bytes"
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

// psStubs replace the cmdlets and the gh command install.ps1 calls; functions
// take precedence over cmdlets and executables in PowerShell. They serve a
// fake release from $env:FIXTURE_DIR for $env:FIXTURE_TAG, log every URL and
// gh call to $env:STUB_LOG (DROP_FIRST_DOWNLOAD=1 fails the first download
// like a dropped connection), and make gh exit with $env:GH_AUTH_EXIT for
// `auth status` and $env:GH_VERIFY_EXIT for `attestation verify`.
// `attestation verify --help` lists the flags in $env:GH_VERIFY_FLAGS
// (default: both flags install.ps1 needs; "none" lists neither).
const psStubs = `$ErrorActionPreference = 'Stop'

function Invoke-WebRequest {
  param([string]$Uri, [string]$OutFile, [string]$Method, [switch]$UseBasicParsing)
  Add-Content -LiteralPath $env:STUB_LOG -Value $Uri
  if ($Uri -like '*/releases/latest') {
    # The final URL after the redirect, as Windows PowerShell reports it.
    $final = [uri]"https://github.com/aarondpn/redmine-cli/releases/tag/$($env:LATEST_TAG)"
    return [pscustomobject]@{ BaseResponse = [pscustomobject]@{ ResponseUri = $final } }
  }
  # DROP_FIRST_DOWNLOAD simulates a transient network failure once.
  $dropped = Join-Path $env:FIXTURE_DIR 'dropped'
  if ($env:DROP_FIRST_DOWNLOAD -eq '1' -and -not (Test-Path -LiteralPath $dropped)) {
    New-Item -ItemType File -Path $dropped | Out-Null
    throw 'The request was aborted: The connection was closed unexpectedly.'
  }
  if ($Uri -notlike "*/releases/download/$($env:FIXTURE_TAG)/*") { throw "404 Not Found: $Uri" }
  $src = Join-Path $env:FIXTURE_DIR ($Uri -split '/')[-1]
  if (-not (Test-Path -LiteralPath $src)) { throw "404 Not Found: $Uri" }
  Copy-Item -LiteralPath $src -Destination $OutFile
}

if ($env:WITH_GH -eq '1') {
  function gh {
    Add-Content -LiteralPath $env:STUB_LOG -Value ('gh ' + ($args -join ' '))
    $a = $args -join ' '
    $global:LASTEXITCODE = 1
    if ($a -eq 'attestation verify --help') {
      'Verify the integrity and provenance of an artifact.'
      'Use the --signer-workflow and --source-ref flags to pin the identity.'
      'FLAGS'
      $flags = 'signer-workflow source-ref'
      if ($env:GH_VERIFY_FLAGS) { $flags = $env:GH_VERIFY_FLAGS }
      foreach ($f in $flags -split ' ') {
        if ($f -and $f -ne 'none') { "      --$f string   description" }
      }
      $global:LASTEXITCODE = 0
    } elseif ($a -eq 'attestation --help') {
      $global:LASTEXITCODE = 0
    } elseif ($a -like 'auth status*') {
      $global:LASTEXITCODE = 0
      if ($env:GH_AUTH_EXIT) { $global:LASTEXITCODE = [int]$env:GH_AUTH_EXIT }
    } elseif ($a -like 'attestation verify *') {
      $global:LASTEXITCODE = 0
      if ($env:GH_VERIFY_EXIT) { $global:LASTEXITCODE = [int]$env:GH_VERIFY_EXIT }
    }
  }
}

# Same shape as ` + "`irm ... | iex`" + `: the script text runs as a script block.
& ([scriptblock]::Create([IO.File]::ReadAllText($env:INSTALL_PS1)))
`

const psArchive = "redmine-cli-windows-amd64.zip"

var psBinary = []byte("fake redmine.exe")

type psEnv struct {
	shell      string
	dir        string
	installDir string
	log        string
	harness    string
}

// powerShells returns the PowerShell executables to test install.ps1 with:
// Windows PowerShell 5.1 (what `irm | iex` gets on a stock Windows) and
// PowerShell 7 when installed.
func powerShells(t *testing.T) []string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("install.ps1 supports Windows only")
	}
	var shells []string
	for _, name := range []string{"powershell", "pwsh"} {
		if p, err := exec.LookPath(name); err == nil {
			shells = append(shells, p)
		}
	}
	if len(shells) == 0 {
		t.Skip("no PowerShell available")
	}
	return shells
}

func newPSEnv(t *testing.T, shell string) *psEnv {
	t.Helper()
	dir := t.TempDir()
	env := &psEnv{
		shell:      shell,
		dir:        dir,
		installDir: filepath.Join(dir, "install dir"),
		log:        filepath.Join(dir, "stub.log"),
		harness:    filepath.Join(dir, "harness.ps1"),
	}
	fixtureDir := filepath.Join(dir, "fixture")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(env.harness, []byte(psStubs), 0o644); err != nil {
		t.Fatal(err)
	}

	archive := buildZip(t, "redmine.exe", psBinary)
	if err := os.WriteFile(filepath.Join(fixtureDir, psArchive), archive, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	env.writeChecksums(t, hex.EncodeToString(sum[:]))
	return env
}

func buildZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e *psEnv) writeChecksums(t *testing.T, sum string) {
	t.Helper()
	name := "redmine-cli_" + strings.TrimPrefix(scriptFixtureTag, "v") + "_checksums.txt"
	sums := fmt.Sprintf("%s  redmine-cli-linux-amd64.tar.gz\n%s  %s\n", strings.Repeat("1", 64), sum, psArchive)
	if err := os.WriteFile(filepath.Join(e.dir, "fixture", name), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
}

// run executes install.ps1 through the harness. The real gh and any GitHub
// tokens or REDMINE_* settings of the test runner are kept out.
func (e *psEnv) run(t *testing.T, withGh bool, extra map[string]string) (string, error) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(e.shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", e.harness)
	for _, kv := range os.Environ() {
		name := strings.ToUpper(strings.SplitN(kv, "=", 2)[0])
		switch {
		// PSModulePath set by the PowerShell 7 that runs CI would make
		// Windows PowerShell 5.1 load the wrong modules (no Get-FileHash).
		case name == "PATH", name == "PSMODULEPATH", name == "GH_TOKEN", name == "GITHUB_TOKEN", strings.HasPrefix(name, "REDMINE_"):
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	ghFlag := "0"
	if withGh {
		ghFlag = "1"
	}
	cmd.Env = append(cmd.Env,
		"PATH="+filepath.Join(os.Getenv("SystemRoot"), "System32"),
		"INSTALL_PS1="+script,
		"REDMINE_INSTALL_DIR="+e.installDir,
		"REDMINE_CLI_NO_MODIFY_PATH=1",
		"STUB_LOG="+e.log,
		"FIXTURE_DIR="+filepath.Join(e.dir, "fixture"),
		"FIXTURE_TAG="+scriptFixtureTag,
		"LATEST_TAG="+scriptFixtureTag,
		"WITH_GH="+ghFlag,
	)
	for k, v := range extra {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (e *psEnv) installed(t *testing.T) bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.installDir, "redmine.exe"))
	if err != nil {
		return false
	}
	return bytes.Equal(data, psBinary)
}

func (e *psEnv) stubLog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallPowerShellScript(t *testing.T) {
	verifyArgs := psArchive + " --repo aarondpn/redmine-cli" +
		" --signer-workflow aarondpn/redmine-cli/.github/workflows/release.yml" +
		" --source-ref refs/tags/" + scriptFixtureTag

	tests := []struct {
		name          string
		withGh        bool
		corruptSums   bool
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
			wantOutput: []string{
				"Latest version: " + scriptFixtureTag, "Checksum verified.", "Attestation verified.",
				"is not in your PATH (REDMINE_CLI_NO_MODIFY_PATH is set)", "redmine.exe' auth login",
				"https://redmine-cli.dev/getting-started/configuration/",
			},
			wantLog: []string{"/releases/latest", "/releases/download/" + scriptFixtureTag + "/" + psArchive, verifyArgs},
		},
		{
			name:          "pinned version without v prefix",
			withGh:        true,
			env:           map[string]string{"REDMINE_CLI_VERSION": strings.TrimPrefix(scriptFixtureTag, "v")},
			wantInstalled: true,
			wantOutput:    []string{"Requested version: " + scriptFixtureTag},
			wantLogNot:    []string{"/releases/latest"},
		},
		{
			name:       "failed attestation refuses to install",
			withGh:     true,
			env:        map[string]string{"GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"Attestation verification failed", "REDMINE_CLI_SKIP_ATTESTATION=1", "Releases older than v2.11.2 have no attestations"},
		},
		{
			name:          "gh without --source-ref falls back to manual verification",
			withGh:        true,
			env:           map[string]string{"GH_VERIFY_FLAGS": "signer-workflow", "GH_VERIFY_EXIT": "1"},
			wantInstalled: true,
			wantOutput:    []string{"lacks --signer-workflow or --source-ref", "gh attestation verify "},
			wantLogNot:    []string{"zip --repo"},
		},
		{
			// Flag names mentioned only in the help prose do not count.
			name:          "gh listing neither flag falls back to manual verification",
			withGh:        true,
			env:           map[string]string{"GH_VERIFY_FLAGS": "none", "GH_VERIFY_EXIT": "1"},
			wantInstalled: true,
			wantOutput:    []string{"lacks --signer-workflow or --source-ref"},
			wantLogNot:    []string{"zip --repo"},
		},
		{
			name:          "REDMINE_CLI_SKIP_ATTESTATION skips gh with a warning",
			withGh:        true,
			env:           map[string]string{"REDMINE_CLI_SKIP_ATTESTATION": "true", "GH_VERIFY_EXIT": "1"},
			wantInstalled: true,
			wantOutput:    []string{"REDMINE_CLI_SKIP_ATTESTATION is set", "a tampered release would NOT be detected", "gh attestation verify "},
			wantLogNot:    []string{"gh attestation verify", "gh auth status"},
		},
		{
			name:       "REDMINE_CLI_SKIP_ATTESTATION=0 still verifies",
			withGh:     true,
			env:        map[string]string{"REDMINE_CLI_SKIP_ATTESTATION": "0", "GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"Attestation verification failed"},
		},
		{
			name:        "skipping the attestation keeps the checksum check",
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
			wantLogNot:    []string{"zip --repo"},
		},
		{
			name:       "token skips the login check",
			withGh:     true,
			env:        map[string]string{"GH_AUTH_EXIT": "1", "GH_TOKEN": "x", "GH_VERIFY_EXIT": "1"},
			wantErr:    true,
			wantOutput: []string{"Attestation verification failed"},
		},
		{
			name:          "a dropped connection is retried",
			env:           map[string]string{"DROP_FIRST_DOWNLOAD": "1"},
			wantInstalled: true,
			wantOutput:    []string{"connection was closed unexpectedly", "retrying...", "Checksum verified."},
		},
		{
			name:       "invalid pinned tag",
			env:        map[string]string{"REDMINE_CLI_VERSION": "v1;calc"},
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

	for _, shell := range powerShells(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					env := newPSEnv(t, shell)
					if tt.corruptSums {
						env.writeChecksums(t, strings.Repeat("0", 64))
					}
					out, err := env.run(t, tt.withGh, tt.env)
					if tt.wantErr != (err != nil) {
						t.Fatalf("err = %v, wantErr %v\noutput:\n%s", err, tt.wantErr, out)
					}
					if got := env.installed(t); got != tt.wantInstalled {
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
		})
	}
}

// An existing installation is replaced; the previous binary does not linger.
func TestInstallPowerShellScript_ReplacesExisting(t *testing.T) {
	for _, shell := range powerShells(t) {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			env := newPSEnv(t, shell)
			if err := os.MkdirAll(env.installDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(env.installDir, "redmine.exe"), []byte("old"), 0o755); err != nil {
				t.Fatal(err)
			}
			out, err := env.run(t, false, nil)
			if err != nil {
				t.Fatalf("install over an existing binary: %v\n%s", err, out)
			}
			if !env.installed(t) {
				t.Errorf("redmine.exe was not replaced\n%s", out)
			}
			if _, err := os.Stat(filepath.Join(env.installDir, "redmine.exe.old")); !os.IsNotExist(err) {
				t.Errorf("redmine.exe.old left behind: %v", err)
			}
		})
	}
}

// The user PATH lives in the registry, so this only runs on CI runners,
// whose registry is thrown away; it still removes its entry afterwards.
func TestInstallPowerShellScript_AddsToUserPath(t *testing.T) {
	shells := powerShells(t)
	if os.Getenv("CI") == "" {
		t.Skip("modifies the user PATH in the registry; runs on CI only")
	}
	shell := shells[0]
	env := newPSEnv(t, shell)
	t.Cleanup(func() {
		script := `$k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
$p = $k.GetValue('Path', '', 'DoNotExpandEnvironmentNames')
$keep = @($p -split ';' | Where-Object { $_ -and $_ -ne $env:DROP_DIR })
$k.SetValue('Path', ($keep -join ';'), 'ExpandString'); $k.Close()`
		cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", script)
		cmd.Env = append(os.Environ(), "DROP_DIR="+env.installDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("restoring the user PATH: %v\n%s", err, out)
		}
	})

	out, err := env.run(t, false, map[string]string{"REDMINE_CLI_NO_MODIFY_PATH": "0"})
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Added "+env.installDir+" to your user PATH") {
		t.Errorf("expected the PATH update to be reported, got:\n%s", out)
	}

	read := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command",
		`[Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment').GetValue('Path', '', 'DoNotExpandEnvironmentNames')`)
	userPath, err := read.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(userPath), env.installDir) {
		t.Errorf("user PATH does not contain %q: %s", env.installDir, userPath)
	}

	// A second install finds the entry and does not add it again.
	out, err = env.run(t, false, map[string]string{"REDMINE_CLI_NO_MODIFY_PATH": "0"})
	if err != nil {
		t.Fatalf("second install: %v\n%s", err, out)
	}
	if strings.Contains(out, "to your user PATH") {
		t.Errorf("second install added the directory again:\n%s", out)
	}
}
