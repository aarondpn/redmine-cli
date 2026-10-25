package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
	"github.com/zalando/go-keyring"
)

func TestKeyringProfileStoresMCPTokenInKeyring(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	cfg := &Config{
		Server:          "https://work.example.com",
		AuthMethod:      "apikey",
		APIKey:          "secret-key",
		CredentialStore: CredentialStoreKeyring,
		MCP:             MCPConfig{AuthToken: "MCP_BEARER_SECRET", HTTP: ":8080"},
	}
	if err := SaveProfile("work", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "MCP_BEARER_SECRET") {
		t.Fatalf("plaintext MCP auth token found in config file:\n%s", data)
	}
	if !strings.Contains(string(data), "http: :8080") {
		t.Fatalf("non-secret mcp settings dropped:\n%s", data)
	}

	stored, ok, err := secrets.Default.Get(keyringIDOf(t, cfgPath, "work"), secrets.FieldMCPToken)
	if err != nil || !ok || stored != "MCP_BEARER_SECRET" {
		t.Fatalf("keyring MCP token = %q ok=%v err=%v", stored, ok, err)
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MCP.AuthToken != "MCP_BEARER_SECRET" {
		t.Fatalf("loaded MCP.AuthToken = %q, want keyring value", loaded.MCP.AuthToken)
	}
}

func TestKeyringProfileWithoutMCPTokenLoads(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "secret-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatalf("Load error = %v, want nil (the MCP token is optional)", err)
	}
	if loaded.MCP.AuthToken != "" {
		t.Fatalf("MCP.AuthToken = %q, want empty", loaded.MCP.AuthToken)
	}
}

func TestKeyringMCPTokenLookupFailureIsNotFatal(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain locked"))
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: fail-id
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REDMINE_API_KEY", "env-key")

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if loaded.MCP.AuthToken != "" {
		t.Fatalf("MCP.AuthToken = %q, want empty", loaded.MCP.AuthToken)
	}
}

func TestKeyringMCPTokenPrecedence(t *testing.T) {
	keyring.MockInit()
	if err := secrets.Default.Set("tok-id", secrets.FieldAPIKey, "kr-key"); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Default.Set("tok-id", secrets.FieldMCPToken, "keyring-token"); err != nil {
		t.Fatal(err)
	}
	base := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: tok-id
`
	cases := []struct {
		name string
		yaml string
		env  string
		want string
	}{
		{name: "keyring fills empty token", want: "keyring-token"},
		{name: "plaintext token in file wins", yaml: "    mcp:\n      auth_token: file-token\n", want: "file-token"},
		{name: "environment wins", env: "env-token", want: "env-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(cfgPath, []byte(base+tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.env != "" {
				t.Setenv("REDMINE_MCP_AUTH_TOKEN", tc.env)
			}
			loaded, err := Load(cfgPath, "", debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			if loaded.MCP.AuthToken != tc.want {
				t.Fatalf("MCP.AuthToken = %q, want %q", loaded.MCP.AuthToken, tc.want)
			}
		})
	}
}

func TestPlaintextMCPTokenMigratesToKeyringOnNextSave(t *testing.T) {
	keyring.MockInit()
	if err := secrets.Default.Set("mig-id", secrets.FieldAPIKey, "kr-key"); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: mig-id
    mcp:
      auth_token: legacy-token
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SetActiveProfile("work", cfgPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "legacy-token") {
		t.Fatalf("MCP token still in plaintext after save:\n%s", data)
	}
	stored, ok, err := secrets.Default.Get("mig-id", secrets.FieldMCPToken)
	if err != nil || !ok || stored != "legacy-token" {
		t.Fatalf("keyring MCP token = %q ok=%v err=%v", stored, ok, err)
	}
	if apiKey, _, _ := secrets.Default.Get("mig-id", secrets.FieldAPIKey); apiKey != "kr-key" {
		t.Fatalf("migration clobbered the API key: %q", apiKey)
	}
	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MCP.AuthToken != "legacy-token" {
		t.Fatalf("MCP.AuthToken after migration = %q", loaded.MCP.AuthToken)
	}
}

func TestMCPTokenKeyringFailureRestoresPlaintextToken(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain locked"))
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	plain := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "plain-key", MCP: MCPConfig{AuthToken: "plain-token"}}
	if err := SaveProfile("work", plain, cfgPath); err != nil {
		t.Fatal(err)
	}

	convert := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "plain-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", convert, cfgPath); err == nil {
		t.Fatal("SaveProfile error = nil, want keyring failure")
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MCP.AuthToken != "plain-token" {
		t.Fatalf("plaintext MCP token lost after failed conversion: %q", loaded.MCP.AuthToken)
	}
}

func TestReloginKeepsMCPSettingsAndKeyringToken(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	first := &Config{
		Server:          "https://work.example.com",
		AuthMethod:      "apikey",
		APIKey:          "old-key",
		CredentialStore: CredentialStoreKeyring,
		MCP:             MCPConfig{AuthToken: "tok", EnableGroups: []string{"issues"}},
	}
	if err := SaveProfile("work", first, cfgPath); err != nil {
		t.Fatal(err)
	}

	// auth login passes a config without an mcp block.
	relogin := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "new-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", relogin, cfgPath); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "new-key" {
		t.Fatalf("APIKey = %q, want %q", loaded.APIKey, "new-key")
	}
	if loaded.MCP.AuthToken != "tok" {
		t.Fatalf("MCP.AuthToken = %q, want keyring token kept across re-login", loaded.MCP.AuthToken)
	}
	if len(loaded.MCP.EnableGroups) != 1 || loaded.MCP.EnableGroups[0] != "issues" {
		t.Fatalf("MCP.EnableGroups = %v, want [issues]", loaded.MCP.EnableGroups)
	}
}

func TestPlaintextSwitchMovesMCPTokenToFile(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	kr := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "kr-key", CredentialStore: CredentialStoreKeyring, MCP: MCPConfig{AuthToken: "tok"}}
	if err := SaveProfile("work", kr, cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")

	plain := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "plain-key"}
	if err := SaveProfile("work", plain, cfgPath); err != nil {
		t.Fatal(err)
	}

	if _, ok, _ := secrets.Default.Get(id, secrets.FieldMCPToken); ok {
		t.Fatal("MCP token orphaned in keyring after switch to plaintext")
	}
	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MCP.AuthToken != "tok" {
		t.Fatalf("MCP.AuthToken = %q, want token moved back into the file", loaded.MCP.AuthToken)
	}
}

func TestDeleteProfileRemovesKeyringMCPToken(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "k", CredentialStore: CredentialStoreKeyring, MCP: MCPConfig{AuthToken: "tok"}}
	if err := SaveProfile("work", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")

	if err := DeleteProfile("work", cfgPath); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := secrets.Default.Get(id, secrets.FieldMCPToken); ok {
		t.Fatal("MCP token left in keyring after profile deletion")
	}
}

func TestPlaintextMCPTokenStaysWhenKeyringUnavailable(t *testing.T) {
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: mig-id
    mcp:
      auth_token: legacy-token
  other:
    server: https://other.example.com
    auth_method: apikey
    api_key: other-key
`
	cases := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{name: "REDMINE_NO_KEYRING", setup: func(t *testing.T) {
			keyring.MockInit()
			t.Setenv("REDMINE_NO_KEYRING", "1")
		}},
		{name: "keyring error", setup: func(t *testing.T) {
			keyring.MockInitWithError(errors.New("keychain locked"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}

			// An unrelated save must keep working, as it did before the
			// token moved to the keyring.
			if err := SetActiveProfile("other", cfgPath); err != nil {
				t.Fatalf("SetActiveProfile error = %v, want nil", err)
			}

			pc, err := LoadProfiles(cfgPath, debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			if pc.ActiveProfile != "other" {
				t.Fatalf("ActiveProfile = %q, want %q", pc.ActiveProfile, "other")
			}
			if got := pc.Profiles["work"].MCP.AuthToken; got != "legacy-token" {
				t.Fatalf("plaintext MCP token = %q, want it kept in the file", got)
			}
		})
	}
}
