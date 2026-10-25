package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
	"github.com/zalando/go-keyring"
)

func TestServerOrigin(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://redmine.example.com", "https://redmine.example.com"},
		{"https://Redmine.Example.com/", "https://redmine.example.com"},
		{"https://redmine.example.com:443/redmine", "https://redmine.example.com"},
		{"http://redmine.example.com:80", "http://redmine.example.com"},
		{"http://redmine.example.com:8080", "http://redmine.example.com:8080"},
		{"HTTPS://redmine.example.com", "https://redmine.example.com"},
		{"redmine.example.com", "https://redmine.example.com"},
		{"redmine.example.com:3000", "https://redmine.example.com:3000"},
		{"http://[::1]:3000", "http://[::1]:3000"},
		{"http://[::1]", "http://[::1]"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ServerOrigin(tc.in); got != tc.want {
			t.Errorf("ServerOrigin(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSameOrigin(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://redmine.example.com", "https://redmine.example.com/", true},
		{"https://redmine.example.com", "https://REDMINE.example.com:443", true},
		{"https://redmine.example.com/a", "https://redmine.example.com/b", true},
		{"https://redmine.example.com", "http://redmine.example.com", false},
		{"https://redmine.example.com", "https://redmine.example.com:8443", false},
		{"https://redmine.example.com", "https://attacker.example", false},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := SameOrigin(tc.a, tc.b); got != tc.want {
			t.Errorf("SameOrigin(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestStoredCredentialServer(t *testing.T) {
	keyring.MockInit()
	if err := secrets.Default.Set("kr-id", secrets.FieldAPIKey, "kr-key"); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Default.Set("kr-basic-id", secrets.FieldPassword, "kr-pw"); err != nil {
		t.Fatal(err)
	}
	content := `active_profile: keyring
profiles:
  keyring:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: kr-id
  plain:
    server: https://plain.example.com
    auth_method: apikey
    api_key: plain-key
  basic:
    server: https://basic.example.com
    auth_method: basic
    username: user
    credential_store: keyring
    keyring_id: kr-basic-id
  noserver:
    auth_method: apikey
    api_key: plain-key
  nokey:
    server: https://nokey.example.com
    auth_method: apikey
`
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		profile    string
		env        map[string]string
		ov         Overrides
		wantOK     bool
		wantServer string
	}{
		{name: "keyring secret with --server", profile: "keyring", ov: Overrides{Server: "https://attacker.example"}, wantOK: true, wantServer: "https://work.example.com"},
		{name: "keyring secret with REDMINE_SERVER", profile: "keyring", env: map[string]string{"REDMINE_SERVER": "https://attacker.example"}, wantOK: true, wantServer: "https://work.example.com"},
		{name: "plaintext secret with --server", profile: "plain", ov: Overrides{Server: "https://attacker.example"}, wantOK: true, wantServer: "https://plain.example.com"},
		{name: "keyring password with --server", profile: "basic", ov: Overrides{Server: "https://attacker.example"}, wantOK: true, wantServer: "https://basic.example.com"},
		{name: "no override still reports binding", profile: "keyring", wantOK: true, wantServer: "https://work.example.com"},
		{name: "explicit --api-key", profile: "keyring", ov: Overrides{Server: "https://attacker.example", APIKey: "flag-key"}},
		{name: "explicit REDMINE_API_KEY", profile: "plain", env: map[string]string{"REDMINE_SERVER": "https://attacker.example", "REDMINE_API_KEY": "env-key"}},
		{name: "explicit REDMINE_PASSWORD", profile: "basic", env: map[string]string{"REDMINE_SERVER": "https://attacker.example", "REDMINE_PASSWORD": "env-pw"}},
		{name: "profile without server", profile: "noserver", ov: Overrides{Server: "https://attacker.example"}},
		{name: "profile without secret", profile: "nokey", ov: Overrides{Server: "https://attacker.example"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cfg, err := LoadWithOverrides(cfgPath, tc.profile, tc.ov, debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			profile, server, ok := cfg.StoredCredentialServer()
			if ok != tc.wantOK {
				t.Fatalf("StoredCredentialServer ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if profile != tc.profile || server != tc.wantServer {
				t.Fatalf("StoredCredentialServer = (%q, %q), want (%q, %q)", profile, server, tc.profile, tc.wantServer)
			}
		})
	}
}

func TestStoredCredentialServerWithoutConfigFile(t *testing.T) {
	t.Setenv("REDMINE_SERVER", "https://env.example.com")
	t.Setenv("REDMINE_API_KEY", "env-key")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"), "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := cfg.StoredCredentialServer(); ok {
		t.Fatal("environment-only config reported a stored credential")
	}
}
