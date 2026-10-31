//go:build e2e

package e2e

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// adminScopes adds the admin scope to the CLI's default set so the oauth
// profile acts as the e2e admin (a token without it acts as a plain user).
const adminScopes = "default,admin"

// loggedInOAuthRunner returns a runner with an admin-scoped oauth login.
func loggedInOAuthRunner(t *testing.T) (*cliRunner, oauthApp) {
	t.Helper()
	wide, _ := requireOAuth(t)
	r := newOAuthRunner(t)
	oauthLogin(t, r, wide, "--scopes", adminScopes)
	return r, wide
}

func TestOAuth_LoginStatusAndCommands(t *testing.T) {
	r, _ := loggedInOAuthRunner(t)

	var status struct {
		Active       bool   `json:"active"`
		Profile      string `json:"profile"`
		AuthMethod   string `json:"auth_method"`
		User         string `json:"user"`
		TokenExpires string `json:"token_expires"`
		Scopes       string `json:"scopes"`
	}
	r.runJSON(t, &status, "auth", "status")
	if !status.Active || status.AuthMethod != "oauth" || status.Profile != oauthProfile {
		t.Fatalf("auth status = %+v, want an active oauth profile %q", status, oauthProfile)
	}
	if !strings.Contains(status.User, e2eUsername()) {
		t.Fatalf("auth status user = %q, want it to name %q", status.User, e2eUsername())
	}
	for _, scope := range []string{"view_issues", "add_issues", "admin"} {
		if !strings.Contains(" "+status.Scopes+" ", " "+scope+" ") {
			t.Fatalf("auth status scopes = %q, want %s", status.Scopes, scope)
		}
	}
	if exp, err := time.Parse(time.RFC3339, status.TokenExpires); err != nil || !exp.After(time.Now()) {
		t.Fatalf("auth status token_expires = %q, want a future time (%v)", status.TokenExpires, err)
	}

	stored := readStoredOAuth(t, r)
	if stored.AccessToken == "" || stored.RefreshToken == "" || stored.ClientSecret == "" {
		t.Fatalf("login did not store the client secret and tokens: %+v", stored)
	}

	var me struct {
		Login string `json:"login"`
		Admin bool   `json:"admin"`
	}
	r.runJSON(t, &me, "users", "me")
	if me.Login != e2eUsername() || !me.Admin {
		t.Fatalf("users me = %+v, want admin %q", me, e2eUsername())
	}

	project := createTestProject(t, r)
	issue := createTestIssue(t, r, project.Identifier)
	var issues []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &issues, "issues", "list", "--project", project.Identifier)
	if len(issues) != 1 || issues[0].ID != issue.ID {
		t.Fatalf("issues list = %+v, want issue %d", issues, issue.ID)
	}

	var profiles []struct {
		Name       string `json:"name"`
		AuthMethod string `json:"auth_method"`
	}
	r.runJSON(t, &profiles, "auth", "list")
	found := false
	for _, p := range profiles {
		found = found || p.Name == oauthProfile && p.AuthMethod == "oauth"
	}
	if !found {
		t.Fatalf("auth list = %+v, want the oauth profile", profiles)
	}
}

func TestOAuth_RefreshRotatesTokens(t *testing.T) {
	r, app := loggedInOAuthRunner(t)
	before := readStoredOAuth(t, r)

	expireStoredToken(t, r)
	var me struct {
		Login string `json:"login"`
	}
	r.runJSON(t, &me, "users", "me")
	if me.Login != e2eUsername() {
		t.Fatalf("users me after refresh = %q, want %q", me.Login, e2eUsername())
	}

	after := readStoredOAuth(t, r)
	if after.AccessToken == before.AccessToken || after.RefreshToken == before.RefreshToken {
		t.Fatalf("refresh did not rotate the stored tokens")
	}
	if !after.Expiry.After(time.Now()) {
		t.Fatalf("stored expiry %v is not in the future", after.Expiry)
	}
	if strings.Join(after.Scopes, " ") != strings.Join(before.Scopes, " ") {
		t.Fatalf("scopes changed on refresh: %v -> %v", before.Scopes, after.Scopes)
	}

	// Redmine retires the old refresh token once the new access token is used.
	status, body := oauthEndpointPost(t, app, "token", map[string][]string{
		"grant_type":    {"refresh_token"},
		"refresh_token": {before.RefreshToken},
	})
	if status == 200 || !strings.Contains(body, "invalid_grant") {
		t.Fatalf("old refresh token: HTTP %d %s, want invalid_grant", status, body)
	}
}

func TestOAuth_ConcurrentRefreshDoesNotLogOut(t *testing.T) {
	r, _ := loggedInOAuthRunner(t)
	before := readStoredOAuth(t, r)
	expireStoredToken(t, r)

	const procs = 5
	var wg sync.WaitGroup
	errs := make([]error, procs)
	for i := range procs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, errs[i] = r.runRaw("users", "me")
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("process %d failed: %v", i, err)
		}
	}

	after := readStoredOAuth(t, r)
	if after.RefreshToken == before.RefreshToken {
		t.Fatal("no process refreshed the token")
	}
	// The login still works after the stampede.
	r.run(t, "users", "me")
}

func TestOAuth_RevokedLoginAsksForNewLogin(t *testing.T) {
	r, app := loggedInOAuthRunner(t)
	stored := readStoredOAuth(t, r)

	status, body := oauthEndpointPost(t, app, "revoke", map[string][]string{"token": {stored.RefreshToken}})
	if status != 200 {
		t.Fatalf("revoke: HTTP %d %s", status, body)
	}

	stderr := r.runExpectJSONError(t, "users", "me")
	env := requireErrorEnvelopeMessage(t, stderr)
	if env.Error.Code != "auth_failed" {
		t.Fatalf("error code = %q, want auth_failed\nstderr:\n%s", env.Error.Code, stderr)
	}
	if !strings.Contains(string(stderr), "auth login") {
		t.Fatalf("error does not point at 'redmine auth login':\n%s", stderr)
	}
}

func TestOAuth_LogoutRevokesTokens(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a pseudo-terminal via script(1)")
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script(1) not found; logout asks for confirmation on a terminal")
	}
	r, _ := loggedInOAuthRunner(t)
	stored := readStoredOAuth(t, r)
	if got := bearerStatus(t, stored.AccessToken); got != 200 {
		t.Fatalf("access token before logout: HTTP %d, want 200", got)
	}

	// logout confirms on the terminal; script(1) provides one.
	cli := []string{builtCLIPath, "--config", r.configPath, "auth", "logout"}
	var args []string
	if runtime.GOOS == "darwin" {
		args = append([]string{"-q", "/dev/null"}, cli...)
	} else {
		args = []string{"-qec", strings.Join(cli, " "), "/dev/null"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, script, args...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1", "REDMINE_NO_KEYRING=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start logout: %v", err)
	}
	// Answer the terminal queries (background colour, cursor position) the
	// prompt library sends before it reads keys.
	_, _ = stdin.Write([]byte("\x1b]11;rgb:0000/0000/0000\x07\x1b[1;1R"))
	time.Sleep(2 * time.Second)
	_, _ = stdin.Write([]byte("y"))
	time.Sleep(time.Second)
	_, _ = stdin.Write([]byte("\r"))
	if err := cmd.Wait(); err != nil {
		t.Fatalf("logout: %v\n%s", err, out.String())
	}

	if got := bearerStatus(t, stored.AccessToken); got != 401 {
		t.Fatalf("access token after logout: HTTP %d, want 401\n%s", got, out.String())
	}
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), stored.RefreshToken) || strings.Contains(string(data), stored.AccessToken) {
		t.Fatalf("logout left tokens in the config:\n%s", data)
	}
}

func TestOAuth_InvalidClientSecret(t *testing.T) {
	wide, _ := requireOAuth(t)
	r := newOAuthRunner(t)

	bad := wide
	bad.Secret = "not-the-secret"
	stderr, err := tryOAuthLogin(t, r, bad)
	if err == nil {
		t.Fatalf("login with a wrong secret succeeded\nstderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "client ID or secret") {
		t.Fatalf("stderr does not mention the client ID or secret:\n%s", stderr)
	}
}

func TestOAuth_ScopesBeyondTheApplication(t *testing.T) {
	_, narrow := requireOAuth(t)
	r := newOAuthRunner(t)

	// The narrow application only allows view_issues; the default set asks for
	// more. Redmine answers with an error page in the browser and never
	// redirects back, so the CLI would wait until its timeout.
	_, err := tryOAuthLogin(t, r, narrow)
	var be *browserError
	if !errors.As(err, &be) || !strings.Contains(err.Error(), "scope is invalid") {
		t.Fatalf("login beyond the application's scopes: %v, want Redmine's invalid scope page", err)
	}

	// Asking for what the application allows works.
	oauthLogin(t, r, narrow, "--scopes", "view_issues")
	if got := readStoredOAuth(t, r).Scopes; strings.Join(got, " ") != "view_issues" {
		t.Fatalf("granted scopes = %v, want [view_issues]", got)
	}
}

func TestOAuth_ReadOnlyLoginCannotWrite(t *testing.T) {
	wide, _ := requireOAuth(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	project := createTestProject(t, admin)
	issue := createTestIssue(t, admin, project.Identifier)

	// Without the admin scope the token acts as a plain user, who may read and
	// comment on a public project. Only the scopes decide here.
	rw := newOAuthRunner(t)
	oauthLogin(t, rw, wide)
	rw.run(t, "issues", "comment", issueIDArg(issue.ID), "-m", "from a read-write login")

	ro := newOAuthRunner(t)
	oauthLogin(t, ro, wide, "--read-only")
	for _, scope := range readStoredOAuth(t, ro).Scopes {
		if strings.HasPrefix(scope, "add_") || strings.HasPrefix(scope, "edit_") || strings.HasPrefix(scope, "delete_") {
			t.Fatalf("read-only login was granted %q", scope)
		}
	}
	ro.run(t, "issues", "get", issueIDArg(issue.ID))
	stderr := ro.runExpectJSONError(t, "issues", "comment", issueIDArg(issue.ID), "-m", "from a read-only login")
	env := requireErrorEnvelopeMessage(t, stderr)
	if env.Error.Code != "forbidden" {
		t.Fatalf("error code = %q, want forbidden\nstderr:\n%s", env.Error.Code, stderr)
	}
}

func TestOAuth_APIKeyOverridesProfile(t *testing.T) {
	r, _ := loggedInOAuthRunner(t)

	// A bogus key replaces the oauth login for the call (no Bearer header).
	stdout, stderr, err := r.runRawWithEnv([]string{"REDMINE_API_KEY=bogus"}, "users", "me")
	if err == nil {
		t.Fatalf("users me with a bogus API key succeeded\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	var me struct {
		Login string `json:"login"`
	}
	r.runJSONWithEnv(t, []string{"REDMINE_API_KEY=" + e2eAPIKey()}, &me, "users", "me")
	if me.Login != e2eUsername() {
		t.Fatalf("users me with the API key = %q, want %q", me.Login, e2eUsername())
	}
}

func TestOAuth_MCPStdioFollowsRotatedTokens(t *testing.T) {
	r, _ := loggedInOAuthRunner(t)
	before := readStoredOAuth(t, r)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx)

	call := func() {
		t.Helper()
		out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects"})
		if err != nil {
			t.Fatalf("CallTool list_projects: %v", err)
		}
		if out.IsError {
			t.Fatalf("list_projects returned IsError: %s", structuredJSON(out))
		}
	}
	call()

	// Another process refreshes meanwhile, which retires the tokens the
	// running server holds. It must pick up the stored pair, not log out.
	expireStoredToken(t, r)
	r.run(t, "users", "me")
	if readStoredOAuth(t, r).RefreshToken == before.RefreshToken {
		t.Fatal("the CLI did not refresh the expired token")
	}
	call()
}

func TestOAuth_UnsupportedBelow61(t *testing.T) {
	requireE2E(t)
	if redmineAtLeast(6, 1) {
		t.Skip("Redmine 6.1+ has an OAuth provider")
	}
	r := newOAuthRunner(t)

	stderr, err := tryOAuthLogin(t, r, oauthApp{ID: "id", Secret: "secret"})
	if err == nil {
		t.Fatalf("OAuth login succeeded on Redmine %s\nstderr:\n%s", e2eVersion(), stderr)
	}
	if !strings.Contains(stderr, "Redmine 6.1 or newer") {
		t.Fatalf("stderr does not say OAuth needs Redmine 6.1:\n%s", stderr)
	}
}
