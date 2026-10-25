package cmdutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
	"github.com/zalando/go-keyring"
)

const overrideTestConfig = `active_profile: keyring
profiles:
  keyring:
    server: https://work.example.com
    auth_method: apikey
    credential_store: keyring
    keyring_id: factory-kr-id
  plain:
    server: https://plain.example.com
    auth_method: apikey
    api_key: plain-key
  basic:
    server: https://basic.example.com
    auth_method: basic
    username: user
    password: plain-pw
  noserver:
    auth_method: apikey
    api_key: plain-key
`

func newOverrideTestFactory(t *testing.T) (*Factory, *bytes.Buffer) {
	t.Helper()
	keyring.MockInit()
	if err := secrets.Default.Set("factory-kr-id", secrets.FieldAPIKey, "kr-key"); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	return &Factory{
		ConfigPath: writeConfigFile(t, overrideTestConfig),
		IOStreams:  &IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &errOut},
	}, &errOut
}

func TestFactoryConfig_StoredCredentialOverrideWarning(t *testing.T) {
	cases := []struct {
		name        string
		profile     string
		server      string
		apiKey      string
		env         map[string]string
		wantWarning string
	}{
		{
			name:        "keyring profile with --server",
			profile:     "keyring",
			server:      "https://attacker.example",
			wantWarning: `warning: sending the stored credentials of profile "keyring" (for https://work.example.com) to https://attacker.example; pass --api-key/REDMINE_API_KEY explicitly to silence this`,
		},
		{
			name:        "keyring profile with REDMINE_SERVER",
			profile:     "keyring",
			env:         map[string]string{"REDMINE_SERVER": "http://attacker.example:8080"},
			wantWarning: `(for https://work.example.com) to http://attacker.example:8080`,
		},
		{
			name:        "plaintext profile with --server",
			profile:     "plain",
			server:      "https://attacker.example",
			wantWarning: `sending the stored credentials of profile "plain" (for https://plain.example.com)`,
		},
		{
			name:        "plaintext profile with REDMINE_SERVER",
			profile:     "plain",
			env:         map[string]string{"REDMINE_SERVER": "https://attacker.example"},
			wantWarning: `sending the stored credentials of profile "plain"`,
		},
		{
			name:        "basic auth profile hints at REDMINE_PASSWORD",
			profile:     "basic",
			server:      "https://attacker.example",
			wantWarning: `set REDMINE_PASSWORD explicitly to silence this`,
		},
		{name: "explicit --api-key suppresses", profile: "keyring", server: "https://attacker.example", apiKey: "flag-key"},
		{name: "explicit REDMINE_API_KEY suppresses", profile: "plain", env: map[string]string{"REDMINE_SERVER": "https://attacker.example", "REDMINE_API_KEY": "env-key"}},
		{name: "explicit REDMINE_PASSWORD suppresses", profile: "basic", env: map[string]string{"REDMINE_SERVER": "https://attacker.example", "REDMINE_PASSWORD": "env-pw"}},
		{name: "same origin is not an override", profile: "keyring", server: "https://WORK.example.com:443/"},
		{name: "no override", profile: "keyring"},
		{name: "profile without server", profile: "noserver", server: "https://attacker.example"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			f, errOut := newOverrideTestFactory(t)
			f.ProfileOverride = tc.profile
			f.ServerOverride = tc.server
			f.APIKeyOverride = tc.apiKey

			if _, err := f.Config(); err != nil {
				t.Fatalf("Config() error = %v, want nil outside strict mode", err)
			}
			got := errOut.String()
			if tc.wantWarning == "" {
				if got != "" {
					t.Fatalf("unexpected warning: %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantWarning) {
				t.Fatalf("stderr = %q, want it to contain %q", got, tc.wantWarning)
			}
		})
	}
}

func TestFactoryConfig_StoredCredentialOverrideStrict(t *testing.T) {
	for _, strictSource := range []string{"env", "profile"} {
		t.Run(strictSource, func(t *testing.T) {
			f, errOut := newOverrideTestFactory(t)
			if strictSource == "env" {
				t.Setenv("REDMINE_STRICT", "1")
			} else {
				f.ConfigPath = writeConfigFile(t, strings.Replace(overrideTestConfig, "    keyring_id: factory-kr-id\n", "    keyring_id: factory-kr-id\n    strict: true\n", 1))
			}
			f.ServerOverride = "https://attacker.example"

			cfg, err := f.Config()
			var strictErr *StrictModeError
			if !errors.As(err, &strictErr) {
				t.Fatalf("Config() = %v, %v; want *StrictModeError", cfg, err)
			}
			if !strings.Contains(err.Error(), "https://attacker.example") {
				t.Fatalf("error %q does not name the override target", err)
			}
			if errOut.Len() != 0 {
				t.Fatalf("strict mode also printed a warning: %q", errOut.String())
			}
		})
	}
}

func TestFactoryConfig_StrictAllowsExplicitKeyOverride(t *testing.T) {
	f, _ := newOverrideTestFactory(t)
	t.Setenv("REDMINE_STRICT", "1")
	t.Setenv("REDMINE_API_KEY", "env-key")
	f.ServerOverride = "https://other.example.com"

	cfg, err := f.Config()
	if err != nil {
		t.Fatalf("Config() error = %v, want nil with an explicit key", err)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.APIKey, "env-key")
	}
}

func TestFactoryConfig_APIKeyFile(t *testing.T) {
	cases := []struct {
		name    string
		content string
		stdin   bool
		want    string
		wantErr string
	}{
		{name: "file with trailing newline", content: "file-key\n", want: "file-key"},
		{name: "file with CRLF", content: "file-key\r\n", want: "file-key"},
		{name: "stdin", content: "stdin-key\n", stdin: true, want: "stdin-key"},
		{name: "empty file", content: "\n", wantErr: "is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := newOverrideTestFactory(t)
			f.ProfileOverride = "plain"
			if tc.stdin {
				f.APIKeyFile = "-"
				f.IOStreams.In = strings.NewReader(tc.content)
			} else {
				f.APIKeyFile = filepath.Join(t.TempDir(), "key")
				if err := os.WriteFile(f.APIKeyFile, []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := f.Config()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Config() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.APIKey != tc.want {
				t.Fatalf("APIKey = %q, want %q", cfg.APIKey, tc.want)
			}
		})
	}
}

func TestFactoryConfig_APIKeyFileMissing(t *testing.T) {
	f, _ := newOverrideTestFactory(t)
	f.APIKeyFile = filepath.Join(t.TempDir(), "absent")
	if _, err := f.Config(); err == nil || !strings.Contains(err.Error(), "--api-key-file") {
		t.Fatalf("Config() error = %v, want an --api-key-file read error", err)
	}
}

func TestFactoryConfig_APIKeyAndFileConflict(t *testing.T) {
	f, _ := newOverrideTestFactory(t)
	f.APIKeyOverride = "flag-key"
	f.APIKeyFile = "-"
	if _, err := f.Config(); err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("Config() error = %v, want conflict error", err)
	}
}

func TestFactoryConfig_StrictRejectsAPIKeyFlag(t *testing.T) {
	t.Setenv("REDMINE_STRICT", "1")

	f, _ := newOverrideTestFactory(t)
	f.APIKeyOverride = "flag-key"
	_, err := f.Config()
	var strictErr *StrictModeError
	if !errors.As(err, &strictErr) || !strings.Contains(err.Error(), "--api-key-file") {
		t.Fatalf("Config() error = %v, want *StrictModeError pointing at --api-key-file", err)
	}

	f, _ = newOverrideTestFactory(t)
	f.APIKeyFile = "-"
	f.IOStreams.In = strings.NewReader("stdin-key\n")
	cfg, err := f.Config()
	if err != nil {
		t.Fatalf("Config() with --api-key-file in strict mode = %v, want nil", err)
	}
	if cfg.APIKey != "stdin-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.APIKey, "stdin-key")
	}
}

func TestFactoryConfig_APIKeyFlagSilentWhenNotStrict(t *testing.T) {
	f, errOut := newOverrideTestFactory(t)
	f.APIKeyOverride = "flag-key"
	if _, err := f.Config(); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("--api-key printed a warning outside strict mode: %q", errOut.String())
	}
}
