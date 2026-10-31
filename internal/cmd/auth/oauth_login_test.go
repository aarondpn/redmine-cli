package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth/oauthtest"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// oauthLogin is a login against the fake provider whose "browser" approves
// the authorize URL at once and records it.
type oauthLogin struct {
	srv     *oauthtest.Server
	f       *cmdutil.Factory
	cfgPath string
	opened  []string
}

func newOAuthLogin(t *testing.T, stdin string) *oauthLogin {
	t.Helper()
	l := &oauthLogin{srv: oauthtest.NewServer(t)}
	l.f, l.cfgPath = loginFactory(t, stdin)
	l.f.ServerOverride = l.srv.URL
	orig := openBrowser
	t.Cleanup(func() { openBrowser = orig })
	openBrowser = func(u string) error {
		l.opened = append(l.opened, u)
		l.srv.Approve(t, u)
		return nil
	}
	return l
}

func (l *oauthLogin) secretFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(l.srv.ClientSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (l *oauthLogin) run(t *testing.T, args ...string) error {
	t.Helper()
	return runLoginCmd(l.f, append([]string{"--name", "work", "--client-id", l.srv.ClientID, "--client-secret-file", l.secretFile(t)}, args...)...)
}

func (l *oauthLogin) scopesRequested(t *testing.T) []string {
	t.Helper()
	if len(l.opened) != 1 {
		t.Fatalf("browser opened %d times, want 1", len(l.opened))
	}
	u, err := url.Parse(l.opened[0])
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(u.Query().Get("scope"))
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

func TestOAuthLogin_SavesAndActivatesProfile(t *testing.T) {
	l := newOAuthLogin(t, "")
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	p := loadProfile(t, l.cfgPath, "work")
	if p.AuthMethod != config.AuthMethodOAuth || p.Server != l.srv.URL {
		t.Fatalf("profile = %+v, want oauth on %s", p, l.srv.URL)
	}
	o := p.OAuth
	if o.ClientID != l.srv.ClientID || o.ClientSecret != l.srv.ClientSecret || o.AccessToken == "" || o.RefreshToken == "" {
		t.Fatalf("oauth config = %+v", o)
	}
	if o.Expiry.Before(time.Now()) || len(o.Scopes) == 0 {
		t.Fatalf("expiry %v scopes %v, want a future expiry and scopes", o.Expiry, o.Scopes)
	}
	if pc, _ := config.LoadProfiles(l.cfgPath, nil); pc.ActiveProfile != "work" {
		t.Fatalf("active profile = %q, want work", pc.ActiveProfile)
	}
	if !strings.Contains(testutil.Stderr(l.f), "Opening the browser") || !strings.Contains(testutil.Stderr(l.f), "Connected as") {
		t.Fatalf("stderr = %q", testutil.Stderr(l.f))
	}
}

func TestOAuthLogin_KeyringKeepsSecretsOutOfFile(t *testing.T) {
	keyring.MockInit()
	real := secrets.Default
	l := newOAuthLogin(t, "")
	secrets.Default = real

	if err := l.run(t, "--keyring"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(l.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), l.srv.ClientSecret) || strings.Contains(string(data), "access_token") || strings.Contains(string(data), "refresh_token") {
		t.Fatalf("secrets in config file:\n%s", data)
	}
	if !strings.Contains(string(data), "client_id: "+l.srv.ClientID) {
		t.Fatalf("client_id missing from config file:\n%s", data)
	}
	cfg, err := config.Load(l.cfgPath, "work", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OAuth.ClientSecret != l.srv.ClientSecret || cfg.OAuth.AccessToken == "" || cfg.OAuth.RefreshToken == "" {
		t.Fatalf("keyring secrets not loaded: %+v", cfg.OAuth)
	}
}

func TestOAuthLogin_SecretFromStdin(t *testing.T) {
	l := newOAuthLogin(t, "test-secret\n")
	if err := runLoginCmd(l.f, "--name", "work", "--client-id", l.srv.ClientID, "--client-secret-file", "-"); err != nil {
		t.Fatal(err)
	}
	if got := loadProfile(t, l.cfgPath, "work").OAuth.ClientSecret; got != "test-secret" {
		t.Fatalf("client secret = %q", got)
	}
}

func TestOAuthLogin_Scopes(t *testing.T) {
	l := newOAuthLogin(t, "")
	if err := l.run(t, "--scopes", "default,admin"); err != nil {
		t.Fatal(err)
	}
	scopes := " " + strings.Join(l.scopesRequested(t), " ") + " "
	for _, want := range []string{"admin", "view_issues", "add_issues"} {
		if !strings.Contains(scopes, " "+want+" ") {
			t.Fatalf("scopes %q lack %s", scopes, want)
		}
	}

	l = newOAuthLogin(t, "")
	if err := l.run(t, "--scopes", "view_project view_issues"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(l.scopesRequested(t), ","); got != "view_project,view_issues" {
		t.Fatalf("scopes = %q", got)
	}
}

func TestOAuthLogin_ReadOnlyProfileRequestsReadScopes(t *testing.T) {
	l := newOAuthLogin(t, "")
	yaml := "profiles:\n  work:\n    server: " + l.srv.URL + "\n    api_key: old\n    auth_method: apikey\n    read_only: true\n"
	if err := os.WriteFile(l.cfgPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	scopes := l.scopesRequested(t)
	if strings.Join(scopes, " ") != strings.Join(oauth.DefaultScopes(true), " ") {
		t.Fatalf("scopes = %v, want the read-only defaults", scopes)
	}
	if !loadProfile(t, l.cfgPath, "work").ReadOnly {
		t.Fatal("read_only was dropped")
	}
}

func TestOAuthLogin_NoBrowserPrintsURL(t *testing.T) {
	l := newOAuthLogin(t, "")
	openBrowser = func(string) error {
		t.Fatal("browser opened despite --no-browser")
		return nil
	}
	// Play the user: approve the URL as soon as it is printed.
	w := &urlWatcher{onURL: func(u string) { l.srv.Approve(t, u) }}
	l.f.IOStreams.ErrOut = w
	if err := l.run(t, "--no-browser"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.String(), "Open this URL in a browser to log in:") {
		t.Fatalf("stderr = %q", w.String())
	}
	loadProfile(t, l.cfgPath, "work")
}

// urlWatcher is a stderr that calls onURL for the first URL line written.
type urlWatcher struct {
	mu    sync.Mutex
	b     strings.Builder
	onURL func(string)
	fired bool
}

func (w *urlWatcher) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.b.Write(p)
	fire := ""
	if !w.fired {
		for _, line := range strings.Split(string(p), "\n") {
			if u := strings.TrimSpace(line); strings.HasPrefix(u, "http") {
				fire, w.fired = u, true
			}
		}
	}
	w.mu.Unlock()
	if fire != "" {
		w.onURL(fire)
	}
	return len(p), nil
}

func (w *urlWatcher) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

func TestOAuthLogin_PreflightErrors(t *testing.T) {
	t.Run("unsupported Redmine", func(t *testing.T) {
		old := httptest.NewServer(http.NotFoundHandler())
		defer old.Close()
		l := newOAuthLogin(t, "")
		l.f.ServerOverride = old.URL
		err := l.run(t)
		if err == nil || !strings.Contains(err.Error(), "OAuth requires Redmine 6.1 or newer") {
			t.Fatalf("error = %v", err)
		}
		if len(l.opened) != 0 {
			t.Fatal("browser opened")
		}
		assertNothingWritten(t, l.cfgPath)
	})
	t.Run("wrong client", func(t *testing.T) {
		l := newOAuthLogin(t, "")
		bad := filepath.Join(t.TempDir(), "secret")
		if err := os.WriteFile(bad, []byte("wrong"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := runLoginCmd(l.f, "--name", "work", "--client-id", l.srv.ClientID, "--client-secret-file", bad)
		if !errors.Is(err, oauth.ErrInvalidClient) || !strings.Contains(cmdutil.ErrorHint(err), "client ID and secret") {
			t.Fatalf("error = %v, hint = %q", err, cmdutil.ErrorHint(err))
		}
		if len(l.opened) != 0 {
			t.Fatal("browser opened")
		}
		assertNothingWritten(t, l.cfgPath)
	})
}

func assertNothingWritten(t *testing.T, cfgPath string) {
	t.Helper()
	if _, err := os.Stat(cfgPath); err == nil {
		t.Fatal("config written despite failed login")
	}
}

func TestOAuthLogin_InvalidScopeHint(t *testing.T) {
	l := newOAuthLogin(t, "")
	l.srv.AllowedScopes = []string{"view_issues"}
	err := l.run(t)
	if !errors.Is(err, oauth.ErrInvalidScope) || !strings.Contains(cmdutil.ErrorHint(err), "--scopes") {
		t.Fatalf("error = %v, hint = %q", err, cmdutil.ErrorHint(err))
	}
	assertNothingWritten(t, l.cfgPath)
}

func TestOAuthLogin_TimeoutHint(t *testing.T) {
	l := newOAuthLogin(t, "")
	openBrowser = func(string) error { return nil }
	orig := oauth.LoginTimeout
	t.Cleanup(func() { oauth.LoginTimeout = orig })
	oauth.LoginTimeout = 50 * time.Millisecond

	err := l.run(t)
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for the browser login") {
		t.Fatalf("error = %v", err)
	}
	if hint := cmdutil.ErrorHint(err); !strings.Contains(hint, "http://127.0.0.1/callback") || !strings.Contains(hint, "scopes") {
		t.Fatalf("hint = %q", hint)
	}
	assertNothingWritten(t, l.cfgPath)
}

func TestOAuthLogin_FlagErrors(t *testing.T) {
	l := newOAuthLogin(t, "")
	l.f.APIKeyOverride = "k"
	err := l.run(t)
	if err == nil || !strings.Contains(err.Error(), "--oauth cannot be combined with --api-key or --api-key-file") {
		t.Fatalf("error = %v", err)
	}
}

// Flags that imply --oauth lead to the OAuth hint when the rest is missing
// and there is no terminal to ask on.
func TestOAuthLogin_ImpliedOAuthNeedsTerminal(t *testing.T) {
	for name, args := range map[string][]string{
		"oauth only":     {"--oauth"},
		"client id only": {"--client-id", "x"},
		"scopes only":    {"--scopes", "view_issues"},
		"no browser":     {"--no-browser"},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := loginFactory(t, "")
			f.ServerOverride = "https://redmine.example.com"
			err := runLoginCmd(f, args...)
			if err == nil || !strings.Contains(err.Error(), "--oauth --client-id ID --client-secret-file -") {
				t.Fatalf("error = %v, want loginNeedsTerminalError", err)
			}
		})
	}
}

func TestOAuthLogin_ReloginReusesClient(t *testing.T) {
	l := newOAuthLogin(t, "")
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	first := loadProfile(t, l.cfgPath, "work")

	// A fresh invocation on the same config, with no client flags.
	f, _ := loginFactory(t, "")
	f.ConfigPath = l.cfgPath
	l.opened = nil
	if err := runLoginCmd(f, "--oauth"); err != nil {
		t.Fatal(err)
	}
	second := loadProfile(t, l.cfgPath, "work")
	if len(l.opened) != 1 {
		t.Fatalf("browser opened %d times, want 1", len(l.opened))
	}
	if second.OAuth.ClientID != first.OAuth.ClientID || second.OAuth.ClientSecret != first.OAuth.ClientSecret || second.Server != first.Server {
		t.Fatalf("client not reused: %+v vs %+v", second.OAuth, first.OAuth)
	}
	if second.OAuth.AccessToken == first.OAuth.AccessToken || second.OAuth.RefreshToken == first.OAuth.RefreshToken {
		t.Fatal("tokens were not replaced")
	}
}

func TestOAuthLogin_ReloginKeepsGrantedScopes(t *testing.T) {
	l := newOAuthLogin(t, "")
	if err := l.run(t, "--scopes", "default,admin"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(loadProfile(t, l.cfgPath, "work").OAuth.Scopes, "admin") {
		t.Fatal("first login was not granted admin")
	}

	f, _ := loginFactory(t, "")
	f.ConfigPath = l.cfgPath
	l.opened = nil
	if err := runLoginCmd(f, "--oauth"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(l.scopesRequested(t), "admin") {
		t.Fatalf("re-login asked for %v, want admin kept", l.scopesRequested(t))
	}
}

func TestOAuthLogin_JSONEnvelope(t *testing.T) {
	l := newOAuthLogin(t, "")
	l.f.OutputFormat = "json"
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(l.f)), &env); err != nil {
		t.Fatalf("stdout is not a single JSON document: %v\n%s", err, testutil.Stdout(l.f))
	}
	if env["action"] != "logged_in" {
		t.Fatalf("action = %v, want logged_in: %v", env["action"], env)
	}
}

// writeOAuthProfile stores an oauth profile named work in cfgPath.
func writeOAuthProfile(t *testing.T, cfgPath string, srv *oauthtest.Server, access, refresh string, expiry time.Time) {
	t.Helper()
	cfg := &config.Config{
		Server:     srv.URL,
		AuthMethod: config.AuthMethodOAuth,
		OAuth: config.OAuthConfig{
			ClientID:     srv.ClientID,
			ClientSecret: srv.ClientSecret,
			AccessToken:  access,
			RefreshToken: refresh,
			Expiry:       expiry,
			Scopes:       []string{"view_issues"},
		},
	}
	if err := config.SaveProfile("work", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}
}

func TestRevokeOAuthLogin(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f, cfgPath := loginFactory(t, "")
	access, refresh := srv.Issue([]string{"view_issues"})
	writeOAuthProfile(t, cfgPath, srv, access, refresh, time.Now().Add(time.Hour))

	if err := revokeOAuthLogin(context.Background(), f, cfgPath, "work"); err != nil {
		t.Fatal(err)
	}
	if srv.Revokes.Load() != 1 {
		t.Fatalf("revokes = %d, want 1", srv.Revokes.Load())
	}

	// A rejected revocation is an error for the caller to turn into a warning.
	srv.ClientSecret = "rotated"
	err := revokeOAuthLogin(context.Background(), f, cfgPath, "work")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("error = %v, want HTTP 401", err)
	}
}

func TestStatus_OAuthRows(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f, cfgPath := loginFactory(t, "")
	f.OutputFormat = "json"
	access, refresh := srv.Issue([]string{"view_issues", "view_project"})
	writeOAuthProfile(t, cfgPath, srv, access, refresh, time.Now().Add(time.Hour))

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &got); err != nil {
		t.Fatalf("%v\n%s", err, testutil.Stdout(f))
	}
	if got["auth_method"] != "oauth" || got["active"] != true {
		t.Fatalf("status = %v", got)
	}
	if got["scopes"] != "view_issues" {
		t.Fatalf("scopes = %v, want the profile's scopes", got["scopes"])
	}
	if exp, _ := got["token_expires"].(string); exp == "" || exp == "never" {
		t.Fatalf("token_expires = %v", got["token_expires"])
	}
}

func TestStatus_OAuthTableRows(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f, cfgPath := loginFactory(t, "")
	access, refresh := srv.Issue([]string{"view_issues"})
	writeOAuthProfile(t, cfgPath, srv, access, refresh, time.Time{})

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{"oauth", "Token Expires", "never", "Scopes", "view_issues"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestStatus_OAuthRefreshesExpiredToken(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f, cfgPath := loginFactory(t, "")
	f.OutputFormat = "json"
	access, refresh := srv.Issue([]string{"view_issues"})
	writeOAuthProfile(t, cfgPath, srv, access, refresh, time.Now().Add(time.Hour))
	srv.ExpireAccessTokens()

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &got); err != nil {
		t.Fatalf("%v\n%s", err, testutil.Stdout(f))
	}
	if got["active"] != true || srv.Refreshes.Load() != 1 {
		t.Fatalf("active = %v, refreshes = %d", got["active"], srv.Refreshes.Load())
	}
	saved := loadProfile(t, cfgPath, "work").OAuth
	if saved.AccessToken == access || saved.RefreshToken == refresh {
		t.Fatal("refreshed tokens were not written back")
	}
}

func TestOAuthLogin_ReloginWithRotatedSecret(t *testing.T) {
	l := newOAuthLogin(t, "")
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	l.srv.ClientSecret = "rotated"
	secretFile := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secretFile, []byte("rotated\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, _ := loginFactory(t, "")
	f.ConfigPath = l.cfgPath
	l.opened = nil
	if err := runLoginCmd(f, "--oauth", "--client-secret-file", secretFile); err != nil {
		t.Fatal(err)
	}
	if got := loadProfile(t, l.cfgPath, "work").OAuth.ClientSecret; got != "rotated" {
		t.Fatalf("saved secret = %q, want rotated", got)
	}
}

func TestOAuthLogin_ReloginRevokesPreviousGrant(t *testing.T) {
	l := newOAuthLogin(t, "")
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	first := loadProfile(t, l.cfgPath, "work")

	// Logging in again revokes the replaced refresh token.
	l.f.ServerOverride = l.srv.URL
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	if l.srv.Revokes.Load() != 1 {
		t.Fatalf("revokes = %d, want 1", l.srv.Revokes.Load())
	}
	resp, err := l.srv.Client().PostForm(l.srv.URL+"/oauth/token", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {first.OAuth.RefreshToken},
		"client_id":     {l.srv.ClientID},
		"client_secret": {l.srv.ClientSecret},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("the replaced refresh token still works")
	}
}

func TestOAuthLogin_ReloginIgnoresEnvServer(t *testing.T) {
	l := newOAuthLogin(t, "")
	if err := l.run(t); err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(other.Close)
	t.Setenv("REDMINE_SERVER", other.URL)
	t.Setenv("REDMINE_API_KEY", "envkey")

	f, _ := loginFactory(t, "")
	f.ConfigPath = l.cfgPath
	l.opened = nil
	if err := runLoginCmd(f, "--oauth"); err != nil {
		t.Fatal(err)
	}
	if got := loadProfile(t, l.cfgPath, "work"); got.Server != l.srv.URL || got.AuthMethod != config.AuthMethodOAuth {
		t.Fatalf("profile = %q %q, want the stored server and oauth", got.Server, got.AuthMethod)
	}

	l.srv.Revokes.Store(0)
	if err := revokeOAuthLogin(context.Background(), f, l.cfgPath, "work"); err != nil {
		t.Fatal(err)
	}
	if l.srv.Revokes.Load() != 1 {
		t.Fatalf("revokes at the profile's server = %d, want 1", l.srv.Revokes.Load())
	}
	if hits.Load() != 0 {
		t.Fatalf("the env server got %d requests, want 0", hits.Load())
	}
}
