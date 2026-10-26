package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/zalando/go-keyring"
)

// fakeAttestationVerifier records calls and returns a canned result.
type fakeAttestationVerifier struct {
	err    error
	calls  int
	digest []byte
	tag    string
}

func (f *fakeAttestationVerifier) Verify(_ context.Context, digest []byte, tag string) error {
	f.calls++
	f.digest = digest
	f.tag = tag
	return f.err
}

func stubAttestationVerifier(t *testing.T, v attestationVerifier) {
	t.Helper()
	orig := newAttestationVerifier
	t.Cleanup(func() { newAttestationVerifier = orig })
	newAttestationVerifier = func() attestationVerifier { return v }
}

// updateFixture serves a release with a valid archive and checksums and
// installs into a temp file instead of the test binary.
type updateFixture struct {
	installPath string
	archive     []byte
	downloads   int
}

const fixtureNewBinary = "new-binary"

func newUpdateFixture(t *testing.T) *updateFixture {
	t.Helper()
	stubHomebrew(t, false, nil)

	fx := &updateFixture{}
	assetName := expectedAssetName()
	if strings.HasSuffix(assetName, ".zip") {
		fx.archive = buildZip(t, map[string][]byte{binaryName + ".exe": []byte(fixtureNewBinary)})
	} else {
		fx.archive = buildTarGz(t, map[string][]byte{binaryName: []byte(fixtureNewBinary)})
	}
	sum := sha256.Sum256(fx.archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), assetName)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/archive":
			fx.downloads++
			w.Write(fx.archive)
		case "/checksums":
			fmt.Fprint(w, checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	origFetch := fetchRelease
	t.Cleanup(func() { fetchRelease = origFetch })
	fetchRelease = func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{
			TagName: "v99.0.0",
			Assets: []GithubAsset{
				{Name: assetName, BrowserDownloadURL: ts.URL + "/archive"},
				{Name: expectedChecksumsName("v99.0.0"), BrowserDownloadURL: ts.URL + "/checksums"},
			},
		}, nil
	}

	fx.installPath = filepath.Join(t.TempDir(), binaryName)
	if err := os.WriteFile(fx.installPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	origInstall := resolveInstallPath
	t.Cleanup(func() { resolveInstallPath = origInstall })
	resolveInstallPath = func(string) (string, error) { return fx.installPath, nil }

	return fx
}

func (fx *updateFixture) installed(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(fx.installPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// testWarn returns the warning function NewCmdUpdate wires up, for a
// Factory that writes warnings to errOut.
func testWarn(errOut *bytes.Buffer) func(string) {
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{Out: io.Discard, ErrOut: errOut}}
	return f.Warn
}

func TestRunUpdate_AttestationDecision(t *testing.T) {
	tests := []struct {
		name            string
		verifyErr       error
		opts            updateOptions
		wantErr         bool
		wantErrContains []string
		wantErrNot      []string
		wantVerifyCalls int
		wantInstalled   bool
		wantStderr      string
		wantNoDownload  bool
	}{
		{
			name:            "valid attestation installs",
			wantVerifyCalls: 1,
			wantInstalled:   true,
		},
		{
			name:            "invalid attestation refuses",
			verifyErr:       errors.New("failed to verify certificate identity"),
			wantErr:         true,
			wantErrContains: []string{"attestation verification failed", "certificate identity", "refusing to install v99.0.0"},
			// Never suggest bypassing a verification that actually failed.
			wantErrNot:      []string{"--skip-attestation"},
			wantVerifyCalls: 1,
		},
		{
			name:            "missing attestation refuses",
			verifyErr:       errNoAttestation,
			wantErr:         true,
			wantErrContains: []string{"no attestation found", "refusing to install"},
			wantErrNot:      []string{"--skip-attestation"},
			wantVerifyCalls: 1,
		},
		{
			name:            "unreachable attestation service refuses with hint",
			verifyErr:       &attestationUnavailableError{err: errors.New("dial tcp: connection refused")},
			wantErr:         true,
			wantErrContains: []string{"connection refused", "refusing to install", "--skip-attestation"},
			wantVerifyCalls: 1,
		},
		{
			name:            "skip flag installs with warning",
			verifyErr:       errors.New("must not be called"),
			opts:            updateOptions{SkipAttestation: true},
			wantVerifyCalls: 0,
			wantInstalled:   true,
			wantStderr:      "warning: --skip-attestation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newUpdateFixture(t)
			fake := &fakeAttestationVerifier{err: tt.verifyErr}
			stubAttestationVerifier(t, fake)

			var out, errOut bytes.Buffer
			opts := tt.opts
			opts.Warn = testWarn(&errOut)
			err := runUpdateWithFormat(context.Background(), "1.0.0", &out, &errOut,
				output.NewStdPrinter(&out, &errOut, false, true, output.FormatTable), opts)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				for _, s := range tt.wantErrContains {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("expected error to contain %q, got: %v", s, err)
					}
				}
				for _, s := range tt.wantErrNot {
					if strings.Contains(err.Error(), s) {
						t.Errorf("expected error not to contain %q, got: %v", s, err)
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fake.calls != tt.wantVerifyCalls {
				t.Errorf("verifier calls = %d, want %d", fake.calls, tt.wantVerifyCalls)
			}
			if fake.calls > 0 {
				sum := sha256.Sum256(fx.archive)
				if !bytes.Equal(fake.digest, sum[:]) {
					t.Errorf("verifier got digest %x, want %x", fake.digest, sum)
				}
				if fake.tag != "v99.0.0" {
					t.Errorf("verifier got tag %q, want v99.0.0", fake.tag)
				}
			}

			got := fx.installed(t)
			if tt.wantInstalled && got != fixtureNewBinary {
				t.Errorf("expected new binary to be installed, got %q", got)
			}
			if !tt.wantInstalled && got != "old-binary" {
				t.Errorf("expected binary to stay untouched, got %q", got)
			}
			if tt.wantStderr != "" && !strings.Contains(errOut.String(), tt.wantStderr) {
				t.Errorf("expected stderr to contain %q, got %q", tt.wantStderr, errOut.String())
			}
			if tt.wantNoDownload && fx.downloads != 0 {
				t.Errorf("expected no archive download, got %d", fx.downloads)
			}
		})
	}
}

// TestNewCmdUpdate_SkipAttestationWarns exercises the real Factory wiring:
// --skip-attestation warns and installs, whatever state the config is in.
func TestNewCmdUpdate_SkipAttestationWarns(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string // empty: no config file
		apiKey     string // --api-key
	}{
		{name: "no config file"},
		{name: "unparsable config", configYAML: "profiles: ["},
		{name: "profile with --api-key", configYAML: "profiles:\n  work:\n    server: https://redmine.example.com\n    api_key: stored\n", apiKey: "X"},
	}
	// Keep keyring lookups away from the real system keyring.
	keyring.MockInit()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDMINE_API_KEY", "")
			fx := newUpdateFixture(t)
			fake := &fakeAttestationVerifier{err: errors.New("must not be called")}
			stubAttestationVerifier(t, fake)

			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if tt.configYAML != "" {
				if err := os.WriteFile(configPath, []byte(tt.configYAML), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errOut bytes.Buffer
			f := &cmdutil.Factory{
				ConfigPath:     configPath,
				APIKeyOverride: tt.apiKey,
				IOStreams:      &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
			}
			cmd := NewCmdUpdate(f, "1.0.0")
			cmd.SetArgs([]string{"--skip-attestation"})
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.Count(errOut.String(), "warning: --skip-attestation"); got != 1 {
				t.Errorf("expected one skip warning on stderr, got %q", errOut.String())
			}
			if fake.calls != 0 {
				t.Errorf("verifier must not run with --skip-attestation, got %d calls", fake.calls)
			}
			if got := fx.installed(t); got != fixtureNewBinary {
				t.Errorf("installed binary = %q, want the new binary", got)
			}
		})
	}
}
