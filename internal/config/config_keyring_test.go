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

func keyringIDOf(t *testing.T, path, name string) string {
	t.Helper()
	pc, err := LoadProfiles(path, debug.New(nil))
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	id := pc.Profiles[name].KeyringID
	if id == "" {
		t.Fatalf("profile %q has no keyring id", name)
	}
	return id
}

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

	stored, ok, err := secrets.Default.Get(keyringIDOf(t, cfgPath, "work"), secrets.FieldAPIKey)
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
	if err := secrets.Default.Set("work-id", secrets.FieldAPIKey, "keyring-key"); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: work-id
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
    keyring_id: headless-id
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
    keyring_id: flag-id
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

func TestKeyringProfileMissingSecretReportsClearError(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: absent-id
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath, "", debug.New(nil))
	if err == nil || !strings.Contains(err.Error(), "keyring") {
		t.Fatalf("Load error = %v, want a clear keyring error", err)
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

	id := keyringIDOf(t, cfgPath, "work")

	if err := DeleteProfile("work", cfgPath); err != nil {
		t.Fatal(err)
	}

	_, ok, err := secrets.Default.Get(id, secrets.FieldAPIKey)
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

	stored, ok, err := secrets.Default.Get(keyringIDOf(t, cfgPath, "work"), secrets.FieldAPIKey)
	if err != nil || !ok {
		t.Fatalf("keyring Get ok=%v err=%v", ok, err)
	}
	if stored != "secret-key" {
		t.Fatalf("keyring value after re-save = %q, want %q", stored, "secret-key")
	}
}

func TestReloginReusesKeyringID(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	first := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "key-one", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", first, cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")

	second := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "key-two", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", second, cfgPath); err != nil {
		t.Fatal(err)
	}

	if got := keyringIDOf(t, cfgPath, "work"); got != id {
		t.Fatalf("keyring id changed on re-login: %q -> %q", id, got)
	}
	stored, ok, err := secrets.Default.Get(id, secrets.FieldAPIKey)
	if err != nil || !ok {
		t.Fatalf("keyring Get ok=%v err=%v", ok, err)
	}
	if stored != "key-two" {
		t.Fatalf("keyring value = %q, want updated %q", stored, "key-two")
	}
}

func TestSaveProfilePlaintextRemovesOrphanedKeyringSecret(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	kr := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "kr-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", kr, cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")

	plain := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "plain-key"}
	if err := SaveProfile("work", plain, cfgPath); err != nil {
		t.Fatal(err)
	}

	_, ok, err := secrets.Default.Get(id, secrets.FieldAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("keyring secret orphaned after switching profile to plaintext")
	}
}

func TestCrossConfigSameNameDoNotCollide(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.yaml")
	pathB := filepath.Join(dir, "b.yaml")

	a := &Config{Server: "https://a.example.com", AuthMethod: "apikey", APIKey: "key-a", CredentialStore: CredentialStoreKeyring}
	b := &Config{Server: "https://b.example.com", AuthMethod: "apikey", APIKey: "key-b", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", a, pathA); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile("work", b, pathB); err != nil {
		t.Fatal(err)
	}

	if keyringIDOf(t, pathA, "work") == keyringIDOf(t, pathB, "work") {
		t.Fatal("same-named profiles in different config files share a keyring id")
	}
	loadedA, err := Load(pathA, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loadedA.APIKey != "key-a" {
		t.Fatalf("config A key = %q, want %q (clobbered by config B)", loadedA.APIKey, "key-a")
	}
}

func TestDeleteProfileSurfacesKeyringErrorAfterCommit(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain locked"))
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: locked-id
  keep:
    server: https://keep.example.com
    auth_method: apikey
    api_key: keep-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := DeleteProfile("work", cfgPath); err == nil {
		t.Fatal("DeleteProfile error = nil, want surfaced keyring failure")
	}

	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pc.Profiles["work"]; ok {
		t.Fatal("profile not removed; config change must commit before keyring cleanup")
	}
	if _, ok := pc.Profiles["keep"]; !ok {
		t.Fatal("unrelated profile lost")
	}
}

func TestSaveKeyringFailureCommitsConfigAndIsRetryable(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain locked"))
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	cfg := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "secret-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", cfg, cfgPath); err == nil {
		t.Fatal("SaveProfile error = nil, want keyring failure")
	}

	id := keyringIDOf(t, cfgPath, "work")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-key") {
		t.Fatalf("plaintext api key leaked into config file on keyring failure:\n%s", data)
	}

	keyring.MockInit()
	retry := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "secret-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", retry, cfgPath); err != nil {
		t.Fatal(err)
	}
	if got := keyringIDOf(t, cfgPath, "work"); got != id {
		t.Fatalf("retry generated new keyring id %q, want reuse of %q", got, id)
	}
	stored, ok, err := secrets.Default.Get(id, secrets.FieldAPIKey)
	if err != nil || !ok {
		t.Fatalf("keyring Get ok=%v err=%v", ok, err)
	}
	if stored != "secret-key" {
		t.Fatalf("keyring value = %q, want %q", stored, "secret-key")
	}
}

func TestAuthMethodSwitchRemovesStaleKeyringField(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	first := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "old-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", first, cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")

	second := &Config{Server: "https://work.example.com", AuthMethod: "basic", Username: "user", Password: "pw", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", second, cfgPath); err != nil {
		t.Fatal(err)
	}

	if got := keyringIDOf(t, cfgPath, "work"); got != id {
		t.Fatalf("keyring id changed on auth method switch: %q -> %q", id, got)
	}
	if _, ok, _ := secrets.Default.Get(id, secrets.FieldAPIKey); ok {
		t.Fatal("stale api key entry remains after switching to basic auth")
	}
	stored, ok, err := secrets.Default.Get(id, secrets.FieldPassword)
	if err != nil || !ok {
		t.Fatalf("keyring Get ok=%v err=%v", ok, err)
	}
	if stored != "pw" {
		t.Fatalf("keyring password = %q, want %q", stored, "pw")
	}
}

func TestSetActiveProfileKeepsCounterpartKeyringField(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	cfg := &Config{Server: "https://work.example.com", AuthMethod: "basic", Username: "user", Password: "pw", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")

	if err := SetActiveProfile("work", cfgPath); err != nil {
		t.Fatal(err)
	}

	stored, ok, err := secrets.Default.Get(id, secrets.FieldPassword)
	if err != nil || !ok {
		t.Fatalf("keyring Get ok=%v err=%v", ok, err)
	}
	if stored != "pw" {
		t.Fatalf("keyring password after re-save = %q, want %q", stored, "pw")
	}
}

func TestServerOverrideOnlyReportsMissingKeyringSecret(t *testing.T) {
	keyring.MockInitWithError(errors.New("dbus unavailable"))
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: srv-id
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadWithOverrides(cfgPath, "", Overrides{Server: "https://other.example.com"}, debug.New(nil))
	if err == nil || !strings.Contains(err.Error(), "keyring") {
		t.Fatalf("Load error = %v, want a clear keyring error (only --server supplied)", err)
	}
}
