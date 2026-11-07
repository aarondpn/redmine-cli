package api

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

func TestNewClientCACert(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer ts.Close()

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.crt")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	if err := os.WriteFile(caFile, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	notPEM := filepath.Join(dir, "not.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	get := func(caCert string) error {
		c, err := NewClient(&config.Config{Server: ts.URL, APIKey: "k", CACert: caCert}, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Get(ts.URL)
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}

	if err := get(caFile); err != nil {
		t.Fatalf("with ca_cert: %v", err)
	}
	if err := get(""); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("without ca_cert: err = %v, want certificate error", err)
	}
	if err := get(filepath.Join(dir, "missing.crt")); err == nil || !strings.Contains(err.Error(), "reading ca_cert") {
		t.Fatalf("missing file: err = %v, want reading ca_cert error", err)
	}
	if err := get(notPEM); err == nil || !strings.Contains(err.Error(), "no PEM certificates") {
		t.Fatalf("non-PEM file: err = %v, want no PEM certificates error", err)
	}

	// A leading ~ is the home directory.
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // os.UserHomeDir on Windows
	if err := get("~/ca.crt"); err != nil {
		t.Fatalf("with ca_cert under ~: %v", err)
	}
}
