package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// tlsLoginServer is a TLS server answering /users/current.json, plus a PEM
// file with its certificate.
func tlsLoginServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/current.json" {
			fmt.Fprint(w, `{"user":{"id":1,"login":"jdoe","firstname":"Jane","lastname":"Doe"}}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caFile, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return srv, caFile
}

// unrelatedCAFile writes a freshly generated self-signed CA that signs none of
// the test servers (httptest servers all share one certificate).
func unrelatedCAFile(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Unrelated Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "unrelated-ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadProfile(t *testing.T, cfgPath, name string) config.Config {
	t.Helper()
	pc, err := config.LoadProfiles(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := pc.Profiles[name]
	if !ok {
		t.Fatalf("profile %q not saved: %+v", name, pc.Profiles)
	}
	return p
}

func TestLoginCACertFlag(t *testing.T) {
	t.Setenv("REDMINE_CA_CERT", "")
	srv, caFile := tlsLoginServer(t)

	// Without a CA the connection test fails on the unknown certificate.
	f, _ := loginFactory(t, "")
	f.ServerOverride, f.APIKeyOverride = srv.URL, "secret"
	err := runLoginCmd(f, "--name", "w")
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("login without --ca-cert: err = %v, want certificate error", err)
	}
	if hint := cmdutil.ErrorHint(err); hint != loginUntrustedCertHint {
		t.Fatalf("hint = %q, want the --ca-cert hint", hint)
	}

	// A relative --ca-cert works and is saved as an absolute path.
	f, cfgPath := loginFactory(t, "")
	f.ServerOverride, f.APIKeyOverride = srv.URL, "secret"
	t.Chdir(filepath.Dir(caFile))
	if err := runLoginCmd(f, "--name", "w", "--ca-cert", filepath.Base(caFile)); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(caFile)
	if err != nil {
		t.Fatal(err)
	}
	got := loadProfile(t, cfgPath, "w").CACert
	if !filepath.IsAbs(got) {
		t.Fatalf("ca_cert = %q, want an absolute path", got)
	}
	if resolved, err := filepath.EvalSymlinks(got); err != nil || resolved != want {
		t.Fatalf("ca_cert = %q (resolves to %q, %v), want %q", got, resolved, err, want)
	}

	// Logging in again without the flag keeps it, and uses it for the test.
	f.ServerOverride, f.APIKeyOverride = srv.URL, "new"
	if err := runLoginCmd(f, "--name", "w"); err != nil {
		t.Fatalf("re-login without --ca-cert: %v", err)
	}
	if p := loadProfile(t, cfgPath, "w"); p.CACert != got || p.APIKey != "new" {
		t.Fatalf("profile after re-login = %+v, want ca_cert %q kept and new key", p, got)
	}
}

func TestLoginCACertFlagUnusableFileFailsFast(t *testing.T) {
	dir := t.TempDir()
	notPEM := filepath.Join(dir, "ca.der")
	if err := os.WriteFile(notPEM, []byte{0x30, 0x82, 0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"missing file": filepath.Join(dir, "missing.pem"),
		"not PEM":      notPEM,
		"directory":    dir,
	} {
		t.Run(name, func(t *testing.T) {
			// No server answers at this address: the check must fail first.
			f, cfgPath := loginFactory(t, "")
			f.ServerOverride, f.APIKeyOverride = "https://127.0.0.1:1", "secret"
			err := runLoginCmd(f, "--name", "w", "--ca-cert", path)
			if err == nil || !strings.HasPrefix(err.Error(), "--ca-cert: ") {
				t.Fatalf("err = %v, want --ca-cert error", err)
			}
			if _, statErr := os.Stat(cfgPath); !os.IsNotExist(statErr) {
				t.Fatalf("config written despite the error: %v", statErr)
			}
		})
	}
}

func TestLoginCACertEnvUsedForTestButNotSaved(t *testing.T) {
	srv, caFile := tlsLoginServer(t)
	t.Setenv("REDMINE_CA_CERT", caFile)

	f, cfgPath := loginFactory(t, "")
	f.ServerOverride, f.APIKeyOverride = srv.URL, "secret"
	if err := runLoginCmd(f, "--name", "w"); err != nil {
		t.Fatalf("login with REDMINE_CA_CERT: %v", err)
	}
	if got := loadProfile(t, cfgPath, "w").CACert; got != "" {
		t.Fatalf("ca_cert = %q, want REDMINE_CA_CERT not saved", got)
	}
}

func writeProfile(t *testing.T, cfgPath, server, caCert string) {
	t.Helper()
	yaml := "active_profile: w\nprofiles:\n  w:\n    server: " + server + "\n    api_key: old\n    auth_method: apikey\n    ca_cert: " + caCert + "\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoginCACertEmptyFlagRemovesStaleValue(t *testing.T) {
	t.Setenv("REDMINE_CA_CERT", "")
	srv := loginServer(t)
	stale := filepath.Join(t.TempDir(), "deleted.pem")

	// The profile's ca_cert no longer exists: logging in again fails on it...
	f, cfgPath := loginFactory(t, "")
	writeProfile(t, cfgPath, srv.URL, stale)
	f.ServerOverride, f.APIKeyOverride = srv.URL, "new"
	err := runLoginCmd(f, "--name", "w")
	if err == nil || !strings.HasPrefix(err.Error(), `ca_cert of profile "w": `) {
		t.Fatalf("re-login with a stale ca_cert: err = %v, want ca_cert error", err)
	}
	if hint := cmdutil.ErrorHint(err); !strings.Contains(hint, `--ca-cert=""`) {
		t.Fatalf("hint = %q, want it to mention --ca-cert=\"\"", hint)
	}

	// ...and --ca-cert="" removes it.
	f.ServerOverride, f.APIKeyOverride = srv.URL, "new"
	if err := runLoginCmd(f, "--name", "w", "--ca-cert="); err != nil {
		t.Fatalf("re-login with --ca-cert \"\": %v", err)
	}
	if p := loadProfile(t, cfgPath, "w"); p.CACert != "" || p.APIKey != "new" {
		t.Fatalf("profile = %+v, want ca_cert removed and new key", p)
	}
}

func TestLoginCACertNotCarriedToOtherServer(t *testing.T) {
	t.Setenv("REDMINE_CA_CERT", "")
	srv := loginServer(t)
	_, caFile := tlsLoginServer(t)

	f, cfgPath := loginFactory(t, "")
	writeProfile(t, cfgPath, "https://redmine.corp.example", caFile)
	f.ServerOverride, f.APIKeyOverride = srv.URL, "new"
	if err := runLoginCmd(f, "--name", "w"); err != nil {
		t.Fatal(err)
	}
	if p := loadProfile(t, cfgPath, "w"); p.CACert != "" {
		t.Fatalf("ca_cert = %q, want it dropped for the other server", p.CACert)
	}
	if !strings.Contains(testutil.Stderr(f), "ca_cert was not carried over") {
		t.Fatalf("missing warning:\n%s", testutil.Stderr(f))
	}
}

func TestLoginCACertPrecedence(t *testing.T) {
	srv, caFile := tlsLoginServer(t)
	otherCA := unrelatedCAFile(t)

	t.Run("flag beats REDMINE_CA_CERT", func(t *testing.T) {
		t.Setenv("REDMINE_CA_CERT", otherCA)
		f, cfgPath := loginFactory(t, "")
		f.ServerOverride, f.APIKeyOverride = srv.URL, "secret"
		if err := runLoginCmd(f, "--name", "w", "--ca-cert", caFile); err != nil {
			t.Fatalf("login: %v", err)
		}
		if got := loadProfile(t, cfgPath, "w").CACert; got != caFile {
			t.Fatalf("ca_cert = %q, want %q", got, caFile)
		}
	})

	t.Run("REDMINE_CA_CERT beats the profile for the test, profile value is saved", func(t *testing.T) {
		t.Setenv("REDMINE_CA_CERT", caFile)
		f, cfgPath := loginFactory(t, "")
		writeProfile(t, cfgPath, srv.URL, otherCA)
		f.ServerOverride, f.APIKeyOverride = srv.URL, "new"
		if err := runLoginCmd(f, "--name", "w"); err != nil {
			t.Fatalf("login: %v", err)
		}
		if got := loadProfile(t, cfgPath, "w").CACert; got != otherCA {
			t.Fatalf("ca_cert = %q, want the profile's %q kept", got, otherCA)
		}
	})

	t.Run("unusable REDMINE_CA_CERT fails before connecting", func(t *testing.T) {
		t.Setenv("REDMINE_CA_CERT", filepath.Join(t.TempDir(), "typo.pem"))
		f, _ := loginFactory(t, "")
		f.ServerOverride, f.APIKeyOverride = "https://127.0.0.1:1", "secret"
		err := runLoginCmd(f, "--name", "w")
		if err == nil || !strings.HasPrefix(err.Error(), "REDMINE_CA_CERT: ") {
			t.Fatalf("err = %v, want REDMINE_CA_CERT error", err)
		}
	})
}

func TestLoginWrongCACertGetsWrongCAHint(t *testing.T) {
	t.Setenv("REDMINE_CA_CERT", "")
	srv, _ := tlsLoginServer(t)
	otherCA := unrelatedCAFile(t)

	f, _ := loginFactory(t, "")
	f.ServerOverride, f.APIKeyOverride = srv.URL, "secret"
	err := runLoginCmd(f, "--name", "w", "--ca-cert", otherCA)
	if err == nil {
		t.Fatal("login with the wrong CA succeeded")
	}
	if hint := cmdutil.ErrorHint(err); hint != fmt.Sprintf(loginWrongCAHint, otherCA) {
		t.Fatalf("hint = %q, want the wrong-CA hint", hint)
	}
}
