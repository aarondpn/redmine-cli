package cmdutil

import (
	"bytes"
	"io"
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
			wantWarning: `warning: sending the stored credentials of profile "keyring" (for https://work.example.com) to https://attacker.example; pass --api-key-file or REDMINE_API_KEY explicitly to silence this`,
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
				t.Fatalf("Config() error = %v, want nil", err)
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

// The warning's hint names --api-key-file; following it silences the
// override check.
func TestFactoryConfig_APIKeyFileSilencesOverrideWarning(t *testing.T) {
	f, errOut := newOverrideTestFactory(t)
	f.ServerOverride = "https://other.example.com"
	f.APIKeyFile = "-"
	f.IOStreams.In = strings.NewReader("file-key\n")

	cfg, err := f.Config()
	if err != nil {
		t.Fatalf("Config() error = %v, want nil with --api-key-file", err)
	}
	if cfg.APIKey != "file-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.APIKey, "file-key")
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected warning with --api-key-file: %q", errOut.String())
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

func TestReadSecretFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cases := []struct {
		name    string
		path    string
		stdin   io.Reader
		want    string
		wantErr string
	}{
		{name: "file, whitespace trimmed", path: write("ok", "  secret\r\n"), want: "secret"},
		{name: "stdin", path: "-", stdin: strings.NewReader("from-stdin\n"), want: "from-stdin"},
		{name: "no stdin", path: "-", wantErr: "--secret-file: no standard input available"},
		{name: "empty path", path: "", wantErr: `--secret-file: file path is empty (use "-" to read from standard input)`},
		{name: "missing file", path: filepath.Join(dir, "absent"), wantErr: "--secret-file: open "},
		{name: "empty file", path: write("empty", "\n\n"), wantErr: "--secret-file: " + filepath.Join(dir, "empty") + " is empty"},
		{name: "empty stdin", path: "-", stdin: strings.NewReader(" \n"), wantErr: "--secret-file: standard input is empty"},
		{name: "too large", path: write("big", strings.Repeat("a", maxSecretFileSize+1)), wantErr: "is larger than 65536 bytes"},
		{name: "at the limit", path: write("limit", strings.Repeat("a", maxSecretFileSize)), want: strings.Repeat("a", maxSecretFileSize)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadSecretFile("--secret-file", tc.path, tc.stdin)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ReadSecretFile() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("ReadSecretFile() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFactoryConfig_APIKeyFlagSilent(t *testing.T) {
	f, errOut := newOverrideTestFactory(t)
	f.APIKeyOverride = "flag-key"
	if _, err := f.Config(); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("--api-key printed a warning: %q", errOut.String())
	}
}

const headerOverrideTestConfig = `active_profile: proxied
profiles:
  proxied:
    server: https://work.example.com
    auth_method: apikey
    api_key: plain-key
    headers:
      x-proxy-token: PROXY_SECRET
      User-Agent: Mozilla/5.0
  bare:
    server: https://bare.example.com
    auth_method: apikey
    api_key: plain-key
`

func TestFactoryConfig_ProfileHeadersOverrideWarning(t *testing.T) {
	const headersWarning = `warning: sending the configured headers of profile "proxied" (for https://work.example.com) to https://attacker.example: `
	cases := []struct {
		name        string
		profile     string
		server      string
		headers     []string
		env         map[string]string
		wantWarning string
	}{
		{name: "--server", server: "https://attacker.example", wantWarning: headersWarning + "User-Agent, X-Proxy-Token; pass the headers meant for that server with --header to silence this"},
		{name: "REDMINE_SERVER", env: map[string]string{"REDMINE_SERVER": "https://attacker.example"}, wantWarning: headersWarning + "User-Agent, X-Proxy-Token;"},
		{name: "explicit key still warns about headers", server: "https://attacker.example", env: map[string]string{"REDMINE_API_KEY": "env-key"}, wantWarning: headersWarning + "User-Agent, X-Proxy-Token;"},
		{name: "--header replaces one", server: "https://attacker.example", headers: []string{"X-Proxy-Token: other"}, wantWarning: headersWarning + "User-Agent;"},
		{name: "--header replaces all", server: "https://attacker.example", headers: []string{"x-proxy-token: other", "user-agent: curl"}, env: map[string]string{"REDMINE_API_KEY": "env-key"}},
		{name: "same origin", server: "https://work.example.com:443"},
		{name: "no override"},
		{name: "profile without headers", profile: "bare", server: "https://attacker.example", env: map[string]string{"REDMINE_API_KEY": "env-key"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			var errOut bytes.Buffer
			f := &Factory{
				ConfigPath:      writeConfigFile(t, headerOverrideTestConfig),
				ProfileOverride: tc.profile,
				ServerOverride:  tc.server,
				Headers:         tc.headers,
				IOStreams:       &IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &errOut},
			}
			if _, err := f.Config(); err != nil {
				t.Fatalf("Config() error = %v, want nil", err)
			}
			got := errOut.String()
			if strings.Contains(got, "PROXY_SECRET") || strings.Contains(got, "Mozilla") {
				t.Fatalf("warning leaks a header value: %q", got)
			}
			if tc.wantWarning == "" {
				if strings.Contains(got, "configured headers") {
					t.Fatalf("unexpected headers warning: %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantWarning) {
				t.Fatalf("stderr = %q, want it to contain %q", got, tc.wantWarning)
			}
		})
	}
}
