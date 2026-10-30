package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

type stubStore struct{ available bool }

func (s stubStore) Get(profile, field string) (string, bool, error) { return "", false, nil }
func (s stubStore) Set(profile, field, secret string) error         { return nil }
func (s stubStore) Delete(profile, field string) error              { return nil }
func (s stubStore) Available() bool                                 { return s.available }

func TestResolveKeyringChoiceNonTTY(t *testing.T) {
	orig := secrets.Default
	t.Cleanup(func() { secrets.Default = orig })

	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{IsTTY: false}}
	keyringProfile := &config.Config{CredentialStore: config.CredentialStoreKeyring}

	cases := []struct {
		name      string
		available bool
		existing  *config.Config
		want      bool
		wantErr   bool
	}{
		{name: "new profile stays plaintext", available: true},
		{name: "plaintext profile stays plaintext", available: true, existing: &config.Config{}},
		{name: "keyring profile keeps keyring", available: true, existing: keyringProfile, want: true},
		{name: "keyring profile with unusable backend errors", available: false, existing: keyringProfile, wantErr: true},
		{name: "new profile with unusable backend stays plaintext", available: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			secrets.Default = stubStore{available: tc.available}
			got, err := resolveKeyringChoice(f, "work", tc.existing, false, false, false)
			if tc.wantErr {
				if err == nil {
					t.Fatal("resolveKeyringChoice error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("resolveKeyringChoice = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveKeyringChoiceExplicitFlag(t *testing.T) {
	orig := secrets.Default
	t.Cleanup(func() { secrets.Default = orig })
	secrets.Default = stubStore{available: false}

	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{IsTTY: true}}

	if _, err := resolveKeyringChoice(f, "work", nil, true, true, false); err == nil {
		t.Fatal("resolveKeyringChoice error = nil, want error for --keyring with unusable backend")
	}

	keyringProfile := &config.Config{CredentialStore: config.CredentialStoreKeyring}
	got, err := resolveKeyringChoice(f, "work", keyringProfile, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("resolveKeyringChoice = true, want false: --keyring=false must win over existing keyring profile")
	}
}

// loginServer stubs /users/current.json and /projects/<id>.json.
func loginServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/redmine") {
		case "/users/current.json":
			fmt.Fprint(w, `{"user":{"id":1,"login":"jdoe","firstname":"Jane","lastname":"Doe"}}`)
		case "/projects/web.json":
			fmt.Fprint(w, `{"project":{"id":2,"name":"Web","identifier":"web"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func loginFactory(t *testing.T, stdin string) (*cmdutil.Factory, string) {
	t.Helper()
	orig := secrets.Default
	t.Cleanup(func() { secrets.Default = orig })
	secrets.Default = stubStore{available: false}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(stdin),
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
		},
	}
	return f, cfgPath
}

func runLoginCmd(f *cmdutil.Factory, args ...string) error {
	cmd := NewCmdLogin(f)
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.Execute()
}

func TestLoginNonInteractive_SavesAndActivatesProfile(t *testing.T) {
	srv := loginServer(t)
	f, cfgPath := loginFactory(t, "")
	f.ServerOverride = srv.URL + "/redmine/projects/x/issues"
	f.APIKeyOverride = "secret"

	if err := runLoginCmd(f, "--name", "work", "--default-project", "web"); err != nil {
		t.Fatal(err)
	}
	pc, err := config.LoadProfiles(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := pc.Profiles["work"]
	if !ok {
		t.Fatalf("profile not saved: %+v", pc.Profiles)
	}
	if p.Server != srv.URL+"/redmine" || p.APIKey != "secret" || p.DefaultProject != "web" {
		t.Fatalf("unexpected profile: %+v", p)
	}
	if pc.ActiveProfile != "work" {
		t.Fatalf("active profile = %q, want work", pc.ActiveProfile)
	}
	if !strings.Contains(testutil.Stderr(f), "Using server URL "+srv.URL+"/redmine") {
		t.Fatalf("missing normalization notice:\n%s", testutil.Stderr(f))
	}
}

func TestLoginNonInteractive_DerivedNameAndStdinKey(t *testing.T) {
	srv := loginServer(t)
	f, cfgPath := loginFactory(t, "fromstdin\n")
	f.ServerOverride = srv.URL
	f.APIKeyFile = "-"

	if err := runLoginCmd(f); err != nil {
		t.Fatal(err)
	}
	pc, err := config.LoadProfiles(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	name := config.ProfileNameFromURL(srv.URL)
	p, ok := pc.Profiles[name]
	if !ok || p.APIKey != "fromstdin" || pc.ActiveProfile != name {
		t.Fatalf("profile %q not saved correctly: %+v active=%q", name, pc.Profiles, pc.ActiveProfile)
	}
	if p.CredentialStore == config.CredentialStoreKeyring {
		t.Fatal("credentials stored in keyring, want plaintext")
	}
}

func TestLoginNonInteractive_JSONEnvelope(t *testing.T) {
	srv := loginServer(t)
	f, _ := loginFactory(t, "")
	f.ServerOverride = srv.URL
	f.APIKeyOverride = "k"
	f.OutputFormat = "json"

	if err := runLoginCmd(f, "--name", "w"); err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &env); err != nil {
		t.Fatalf("stdout is not a single JSON document: %v\n%s", err, testutil.Stdout(f))
	}
	if env["action"] != "logged_in" {
		t.Fatalf("action = %v, want logged_in: %v", env["action"], env)
	}
}

func TestLoginNonInteractive_UnknownDefaultProject(t *testing.T) {
	srv := loginServer(t)
	f, cfgPath := loginFactory(t, "")
	f.ServerOverride = srv.URL
	f.APIKeyOverride = "k"

	err := runLoginCmd(f, "--default-project", "nope")
	if err == nil || !strings.Contains(err.Error(), "default project") {
		t.Fatalf("error = %v, want default project error", err)
	}
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		t.Fatal("config written despite failed login")
	}
}

func TestLoginNoTerminalFailsFast(t *testing.T) {
	for name, setup := range map[string]func(f *cmdutil.Factory){
		"nothing":     func(f *cmdutil.Factory) {},
		"server only": func(f *cmdutil.Factory) { f.ServerOverride = "https://redmine.example.com" },
		"nothing with json": func(f *cmdutil.Factory) {
			f.OutputFormat = "json"
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := loginFactory(t, "")
			setup(f)
			err := runLoginCmd(f)
			if err == nil || !strings.Contains(err.Error(), "--api-key-file") || !strings.Contains(err.Error(), "--server") {
				t.Fatalf("error = %v, want loginNeedsTerminalError", err)
			}
		})
	}
}

func TestLoginNonInteractive_KeepsDefaultProject(t *testing.T) {
	srv := loginServer(t)
	for name, tc := range map[string]struct {
		server string
		want   string
	}{
		"same server":  {server: srv.URL, want: "web"},
		"other server": {server: "https://other.example.com", want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			f, cfgPath := loginFactory(t, "")
			yaml := "active_profile: w\nprofiles:\n  w:\n    server: " + tc.server + "\n    api_key: old\n    auth_method: apikey\n    default_project: web\n"
			if err := os.WriteFile(cfgPath, []byte(yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			f.ServerOverride = srv.URL
			f.APIKeyOverride = "new"

			if err := runLoginCmd(f, "--name", "w"); err != nil {
				t.Fatal(err)
			}
			pc, err := config.LoadProfiles(cfgPath, nil)
			if err != nil {
				t.Fatal(err)
			}
			if p := pc.Profiles["w"]; p.APIKey != "new" || p.DefaultProject != tc.want {
				t.Fatalf("profile = %+v, want api key new and default project %q", p, tc.want)
			}
		})
	}
}

func TestLoginNonInteractive_KeyringFlagFalse(t *testing.T) {
	srv := loginServer(t)
	f, cfgPath := loginFactory(t, "")
	secrets.Default = stubStore{available: true}
	f.ServerOverride = srv.URL
	f.APIKeyOverride = "k"

	if err := runLoginCmd(f, "--name", "w", "--keyring=false"); err != nil {
		t.Fatal(err)
	}
	pc, err := config.LoadProfiles(cfgPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p := pc.Profiles["w"]; p.CredentialStore == config.CredentialStoreKeyring || p.APIKey != "k" {
		t.Fatalf("expected plaintext credentials, got %+v", p)
	}
}

func TestLoginIgnoresActiveProfileHeadersAndFormat(t *testing.T) {
	srv := loginServer(t)
	f, cfgPath := loginFactory(t, "")
	yaml := "active_profile: corp\nprofiles:\n  corp:\n    server: https://corp.example.com\n    api_key: x\n    auth_method: apikey\n    output_format: json\n    headers:\n      X-Proxy-Token: t\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	f.ServerOverride = srv.URL
	f.APIKeyOverride = "k"

	if err := runLoginCmd(f, "--name", "local"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(testutil.Stderr(f), "X-Proxy-Token") {
		t.Fatalf("false header warning:\n%s", testutil.Stderr(f))
	}
	if strings.HasPrefix(strings.TrimSpace(testutil.Stdout(f)), "{") {
		t.Fatalf("active profile output_format leaked:\n%s", testutil.Stdout(f))
	}
}
