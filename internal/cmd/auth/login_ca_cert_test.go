package auth

import (
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
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
	if err := runLoginCmd(f, "--name", "w"); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("login without --ca-cert: err = %v, want certificate error", err)
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

func TestLoginCACertFlagMissingFileFailsFast(t *testing.T) {
	f, cfgPath := loginFactory(t, "")
	f.ServerOverride, f.APIKeyOverride = "https://redmine.example.com", "secret"
	err := runLoginCmd(f, "--name", "w", "--ca-cert", filepath.Join(t.TempDir(), "missing.pem"))
	if err == nil || !strings.Contains(err.Error(), "--ca-cert") {
		t.Fatalf("err = %v, want --ca-cert error", err)
	}
	if _, statErr := os.Stat(cfgPath); !os.IsNotExist(statErr) {
		t.Fatalf("config written despite the error: %v", statErr)
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
