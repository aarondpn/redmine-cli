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

func TestKeyringProfileRoundtrip(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	cfg := &Config{
		Server:          "https://work.example.com",
		AuthMethod:      "apikey",
		APIKey:          "secret-key",
		CredentialStore: CredentialStoreKeyring,
	}
	if err := SaveProfile("work", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-key") {
		t.Fatalf("plaintext api key found in config file:\n%s", data)
	}
	if !strings.Contains(string(data), "credential_store: keyring") {
		t.Fatalf("credential_store marker missing:\n%s", data)
	}

	stored, ok, err := secrets.Default.Get("work", secrets.FieldAPIKey)
	if err != nil || !ok {
		t.Fatalf("keyring Get ok=%v err=%v", ok, err)
	}
	if stored != "secret-key" {
		t.Fatalf("keyring value = %q, want %q", stored, "secret-key")
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "secret-key" {
		t.Fatalf("loaded APIKey = %q, want %q", loaded.APIKey, "secret-key")
	}
}

type fatalStore struct{ t *testing.T }

func (s fatalStore) Get(profile, field string) (string, bool, error) {
	s.t.Fatalf("keyring Get called unexpectedly for %s/%s", profile, field)
	return "", false, nil
}

func (s fatalStore) Set(profile, field, secret string) error {
	s.t.Fatalf("keyring Set called unexpectedly for %s/%s", profile, field)
	return nil
}

func (s fatalStore) Delete(profile, field string) error {
	s.t.Fatalf("keyring Delete called unexpectedly for %s/%s", profile, field)
	return nil
}

func (s fatalStore) Available() bool { return false }

func TestLegacyPlaintextProfileUnaffected(t *testing.T) {
	orig := secrets.Default
	secrets.Default = fatalStore{t}
	t.Cleanup(func() { secrets.Default = orig })

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    api_key: plain-key
    auth_method: apikey
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "plain-key" {
		t.Fatalf("loaded APIKey = %q, want %q", loaded.APIKey, "plain-key")
	}
}

func TestEnvVarWinsOverKeyring(t *testing.T) {
	keyring.MockInit()
	if err := secrets.Default.Set("work", secrets.FieldAPIKey, "keyring-key"); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("REDMINE_API_KEY", "env-key")

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "env-key" {
		t.Fatalf("loaded APIKey = %q, want env value %q", loaded.APIKey, "env-key")
	}
}

func TestHeadlessKeyringReturnsClearError(t *testing.T) {
	keyring.MockInitWithError(errors.New("dbus unavailable"))
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath, "", debug.New(nil))
	if err == nil {
		t.Fatal("Load error = nil, want clear keyring error")
	}
	if !strings.Contains(err.Error(), "keyring") {
		t.Fatalf("error %q does not mention keyring", err)
	}
}

func TestEnvVarWinsOverFailingKeyring(t *testing.T) {
	keyring.MockInitWithError(errors.New("dbus unavailable"))
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("REDMINE_API_KEY", "env-key")

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatalf("Load error = %v, want nil (env var supplies the key)", err)
	}
	if loaded.APIKey != "env-key" {
		t.Fatalf("loaded APIKey = %q, want env value %q", loaded.APIKey, "env-key")
	}
}

func TestAPIKeyOverrideBypassesKeyring(t *testing.T) {
	orig := secrets.Default
	secrets.Default = fatalStore{t}
	t.Cleanup(func() { secrets.Default = orig })

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadWithOverrides(cfgPath, "", Overrides{APIKey: "flag-key"}, debug.New(nil))
	if err != nil {
		t.Fatalf("Load error = %v, want nil (flag supplies the key)", err)
	}
	if loaded.APIKey != "flag-key" {
		t.Fatalf("loaded APIKey = %q, want flag value %q", loaded.APIKey, "flag-key")
	}
}

func TestDeleteProfileRemovesKeyringEntries(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	first := &Config{Server: "https://a.example.com", AuthMethod: "apikey", APIKey: "key-a", CredentialStore: CredentialStoreKeyring}
	second := &Config{Server: "https://b.example.com", AuthMethod: "apikey", APIKey: "key-b"}
	if err := SaveProfile("work", first, cfgPath); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile("other", second, cfgPath); err != nil {
		t.Fatal(err)
	}

	if err := DeleteProfile("work", cfgPath); err != nil {
		t.Fatal(err)
	}

	_, ok, err := secrets.Default.Get("work", secrets.FieldAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("keyring entry still present after DeleteProfile")
	}
}

func TestSaveKeyringProfileReloadDoesNotClobber(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	cfg := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "secret-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	if err := SetActiveProfile("work", cfgPath); err != nil {
		t.Fatal(err)
	}

	stored, ok, err := secrets.Default.Get("work", secrets.FieldAPIKey)
	if err != nil || !ok {
		t.Fatalf("keyring Get ok=%v err=%v", ok, err)
	}
	if stored != "secret-key" {
		t.Fatalf("keyring value after re-save = %q, want %q", stored, "secret-key")
	}
}
