package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
	"github.com/zalando/go-keyring"
)

func oauthConfig(store string) *Config {
	return &Config{
		Server:          "https://work.example.com",
		AuthMethod:      AuthMethodOAuth,
		CredentialStore: store,
		OAuth: OAuthConfig{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			Expiry:       time.Date(2026, 10, 31, 14, 5, 0, 0, time.UTC),
			Scopes:       []string{"view_project", "view_issues"},
		},
	}
}

func TestOAuthPlaintextRoundtrip(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	want := oauthConfig("")
	if err := SaveProfile("work", want, cfgPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"oauth:", "client_id: client-id", "refresh_token: refresh-1", "expiry: 2026-10-31T14:05:00Z", "- view_issues"} {
		if !strings.Contains(string(data), s) {
			t.Fatalf("config file missing %q:\n%s", s, data)
		}
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AuthMethod != AuthMethodOAuth {
		t.Fatalf("AuthMethod = %q, want oauth", loaded.AuthMethod)
	}
	if !reflect.DeepEqual(loaded.OAuth, want.OAuth) {
		t.Fatalf("OAuth = %+v, want %+v", loaded.OAuth, want.OAuth)
	}
}

func TestOAuthKeyringRoundtrip(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	want := oauthConfig(CredentialStoreKeyring)
	if err := SaveProfile("work", want, cfgPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"client-secret", "access-1", "refresh-1"} {
		if strings.Contains(string(data), s) {
			t.Fatalf("secret %q found in config file:\n%s", s, data)
		}
	}
	if !strings.Contains(string(data), "client_id: client-id") {
		t.Fatalf("client_id missing from config file:\n%s", data)
	}

	id := keyringIDOf(t, cfgPath, "work")
	for field, value := range map[string]string{
		secrets.FieldOAuthClientSecret: "client-secret",
		secrets.FieldOAuthAccessToken:  "access-1",
		secrets.FieldOAuthRefreshToken: "refresh-1",
	} {
		got, ok, err := secrets.Default.Get(id, field)
		if err != nil || !ok || got != value {
			t.Fatalf("keyring %s = %q ok=%v err=%v, want %q", field, got, ok, err, value)
		}
	}

	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.OAuth, want.OAuth) {
		t.Fatalf("OAuth = %+v, want %+v", loaded.OAuth, want.OAuth)
	}
}

func TestOAuthKeyringMissingRefreshTokenReportsError(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveProfile("work", oauthConfig(CredentialStoreKeyring), cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")
	if err := secrets.Default.Delete(id, secrets.FieldOAuthRefreshToken); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath, "", debug.New(nil))
	if !IsKeyringSecretError(err) {
		t.Fatalf("Load error = %v, want keyring secret error", err)
	}
}

func TestOAuthAuthMethodSwitchRemovesStaleKeyringFields(t *testing.T) {
	oauthFields := []string{secrets.FieldOAuthClientSecret, secrets.FieldOAuthAccessToken, secrets.FieldOAuthRefreshToken}
	apikey := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "key", CredentialStore: CredentialStoreKeyring}
	basic := &Config{Server: "https://work.example.com", AuthMethod: "basic", Username: "user", Password: "pw", CredentialStore: CredentialStoreKeyring}
	oauth := oauthConfig(CredentialStoreKeyring)

	tests := []struct {
		name        string
		first, next *Config
		wantGone    []string
		wantKept    []string
	}{
		{"apikey to oauth", apikey, oauth, []string{secrets.FieldAPIKey}, oauthFields},
		{"basic to oauth", basic, oauth, []string{secrets.FieldPassword}, oauthFields},
		{"oauth to apikey", oauth, apikey, oauthFields, []string{secrets.FieldAPIKey}},
		{"oauth to basic", oauth, basic, oauthFields, []string{secrets.FieldPassword}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyring.MockInit()
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := SaveProfile("work", tt.first, cfgPath); err != nil {
				t.Fatal(err)
			}
			id := keyringIDOf(t, cfgPath, "work")
			if err := SaveProfile("work", tt.next, cfgPath); err != nil {
				t.Fatal(err)
			}
			if got := keyringIDOf(t, cfgPath, "work"); got != id {
				t.Fatalf("keyring id changed on auth method switch: %q -> %q", id, got)
			}
			for _, field := range tt.wantGone {
				if _, ok, _ := secrets.Default.Get(id, field); ok {
					t.Errorf("stale %s entry remains", field)
				}
			}
			for _, field := range tt.wantKept {
				if _, ok, _ := secrets.Default.Get(id, field); !ok {
					t.Errorf("%s entry missing", field)
				}
			}
		})
	}
}

// getCountingStore records which fields are read from the keyring.
type getCountingStore struct {
	secrets.Store
	gets []string
}

func (s *getCountingStore) Get(profile, field string) (string, bool, error) {
	s.gets = append(s.gets, field)
	return s.Store.Get(profile, field)
}

func TestAPIKeyOverrideSwitchesOAuthProfileWithoutKeyringRead(t *testing.T) {
	tests := []struct {
		name string
		ov   Overrides
		env  string
	}{
		{"flag", Overrides{APIKey: "flag-key"}, ""},
		{"env", Overrides{}, "env-key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyring.MockInit()
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := SaveProfile("work", oauthConfig(CredentialStoreKeyring), cfgPath); err != nil {
				t.Fatal(err)
			}

			orig := secrets.Default
			store := &getCountingStore{Store: orig}
			secrets.Default = store
			t.Cleanup(func() { secrets.Default = orig })
			t.Setenv("REDMINE_API_KEY", tt.env)

			loaded, err := LoadWithOverrides(cfgPath, "", tt.ov, debug.New(nil))
			if err != nil {
				t.Fatalf("Load error = %v, want nil", err)
			}
			if loaded.AuthMethod != "apikey" {
				t.Fatalf("AuthMethod = %q, want apikey", loaded.AuthMethod)
			}
			for _, field := range store.gets {
				if strings.HasPrefix(field, "oauth_") {
					t.Fatalf("OAuth keyring field %s was read", field)
				}
			}
			if _, _, ok := loaded.StoredCredentialServer(); ok {
				t.Fatal("StoredCredentialServer ok for a flag/env API key")
			}
		})
	}
}

func TestStoredCredentialServerOAuth(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveProfile("work", oauthConfig(""), cfgPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	profile, server, ok := loaded.StoredCredentialServer()
	if !ok || profile != "work" || server != "https://work.example.com" {
		t.Fatalf("StoredCredentialServer = %q, %q, %v", profile, server, ok)
	}
}

func TestProfileLocation(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveProfile("work", oauthConfig(""), cfgPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	path, profile, ok := loaded.ProfileLocation()
	if !ok || path != cfgPath || profile != "work" {
		t.Fatalf("ProfileLocation = %q, %q, %v", path, profile, ok)
	}

	var none *Config
	if _, _, ok := none.ProfileLocation(); ok {
		t.Fatal("ProfileLocation ok for nil config")
	}
	if _, _, ok := (&Config{}).ProfileLocation(); ok {
		t.Fatal("ProfileLocation ok without a selected profile")
	}
}

func TestSaveOAuthTokens(t *testing.T) {
	newTokens := OAuthConfig{
		ClientID:     "client-id",
		AccessToken:  "access-2",
		RefreshToken: "refresh-2",
		Expiry:       time.Date(2026, 10, 31, 16, 5, 0, 0, time.UTC),
		Scopes:       []string{"view_project"},
	}

	for _, store := range []string{"", CredentialStoreKeyring} {
		name := "plaintext"
		if store != "" {
			name = store
		}
		t.Run(name, func(t *testing.T) {
			keyring.MockInit()
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			other := &Config{Server: "https://other.example.com", AuthMethod: "apikey", APIKey: "other-key"}
			if err := SaveProfile("other", other, cfgPath); err != nil {
				t.Fatal(err)
			}
			if err := SaveProfile("work", oauthConfig(store), cfgPath); err != nil {
				t.Fatal(err)
			}

			if err := SaveOAuthTokens(cfgPath, "work", newTokens); err != nil {
				t.Fatal(err)
			}

			got, err := LoadOAuthTokens(cfgPath, "work")
			if err != nil {
				t.Fatal(err)
			}
			want := newTokens
			want.ClientSecret = "client-secret"
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("LoadOAuthTokens = %+v, want %+v", got, want)
			}

			pc, err := LoadProfiles(cfgPath, debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			if pc.Profiles["other"].APIKey != "other-key" {
				t.Fatalf("other profile changed: %+v", pc.Profiles["other"])
			}
			work := pc.Profiles["work"]
			if work.Server != "https://work.example.com" || work.OAuth.ClientID != "client-id" {
				t.Fatalf("work profile changed: %+v", work)
			}
		})
	}
}

func TestSaveOAuthTokensErrors(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveProfile("work", oauthConfig(""), cfgPath); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile("plain", &Config{Server: "https://p.example.com", AuthMethod: "apikey", APIKey: "k"}, cfgPath); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		profile string
		tok     OAuthConfig
		want    string
	}{
		{"missing profile", "gone", OAuthConfig{ClientID: "client-id"}, `profile "gone" not found`},
		{"changed client id", "work", OAuthConfig{ClientID: "other-client"}, `profile "work" changed while refreshing the OAuth token; run the command again`},
		{"changed auth method", "plain", OAuthConfig{ClientID: "client-id"}, `profile "plain" changed while refreshing the OAuth token; run the command again`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SaveOAuthTokens(cfgPath, tt.profile, tt.tok)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDeleteProfileRemovesOAuthKeyringEntries(t *testing.T) {
	keyring.MockInit()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveProfile("work", oauthConfig(CredentialStoreKeyring), cfgPath); err != nil {
		t.Fatal(err)
	}
	id := keyringIDOf(t, cfgPath, "work")

	if err := DeleteProfile("work", cfgPath); err != nil {
		t.Fatal(err)
	}

	for _, field := range []string{secrets.FieldOAuthClientSecret, secrets.FieldOAuthAccessToken, secrets.FieldOAuthRefreshToken} {
		if _, ok, err := secrets.Default.Get(id, field); err != nil || ok {
			t.Fatalf("keyring %s present=%v err=%v after DeleteProfile", field, ok, err)
		}
	}
}
