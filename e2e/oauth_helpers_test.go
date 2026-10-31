//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

// oauthProfile is the profile name oauthLogin saves.
const oauthProfile = "oauth-e2e"

// oauthApp is an OAuth application registered by oauth-app.sh.
type oauthApp struct{ ID, Secret string }

// oauthApps returns the wide and the narrow application from
// REDMINE_E2E_OAUTH_APPS (one "<id> <secret>" line each, set by the Makefile).
func oauthApps(t *testing.T) (wide, narrow oauthApp) {
	t.Helper()
	fields := strings.Fields(os.Getenv("REDMINE_E2E_OAUTH_APPS"))
	if len(fields) != 4 {
		t.Fatalf("REDMINE_E2E_OAUTH_APPS must hold two \"<id> <secret>\" lines, got %d fields", len(fields))
	}
	return oauthApp{fields[0], fields[1]}, oauthApp{fields[2], fields[3]}
}

// requireOAuth skips unless the Redmine under test has an OAuth provider and
// the admin password is available to play the browser.
func requireOAuth(t *testing.T) (wide, narrow oauthApp) {
	t.Helper()
	requireE2E(t)
	skipBelowRedmine(t, 6, 1, "OAuth")
	if e2ePassword() == "" {
		t.Skip("set REDMINE_E2E_PASSWORD to run OAuth tests (see e2e/README.md)")
	}
	return oauthApps(t)
}

// newOAuthRunner returns a runner with an empty config, ready for oauthLogin.
func newOAuthRunner(t *testing.T) *cliRunner {
	t.Helper()
	return newRunnerWithConfig(t, "")
}

// oauthLogin runs `auth login --oauth --no-browser` for app, plays the browser
// (admin login, consent, redirect to the CLI's listener) and returns the
// command's stderr. It fails the test unless the login succeeds.
func oauthLogin(t *testing.T, r *cliRunner, app oauthApp, extraArgs ...string) string {
	t.Helper()
	stderr, err := tryOAuthLogin(t, r, app, extraArgs...)
	if err != nil {
		t.Fatalf("oauth login: %v\nstderr:\n%s", err, stderr)
	}
	return stderr
}

// tryOAuthLogin is oauthLogin for logins that may fail; the error is the
// command's exit error.
func tryOAuthLogin(t *testing.T, r *cliRunner, app oauthApp, extraArgs ...string) (string, error) {
	t.Helper()
	args := []string{"--config", r.configPath, "--output", "json", "auth", "login",
		"--server", e2eBaseURL(), "--oauth", "--no-browser",
		"--client-id", app.ID, "--client-secret-file", "-", "--name", oauthProfile}
	args = append(args, extraArgs...)

	cmd := exec.Command(builtCLIPath, args...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1", "REDMINE_NO_KEYRING=1")
	cmd.Stdin = strings.NewReader(app.Secret + "\n")
	pipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start login: %v", err)
	}

	// Collect stderr and hand over the authorize URL as soon as it shows up.
	var (
		mu      sync.Mutex
		buf     bytes.Buffer
		authURL = make(chan string, 1)
		done    = make(chan struct{})
	)
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			line := sc.Text()
			mu.Lock()
			buf.WriteString(line + "\n")
			mu.Unlock()
			if u := strings.TrimSpace(line); strings.HasPrefix(u, e2eBaseURL()) && strings.Contains(u, "/oauth/authorize") {
				select {
				case authURL <- u:
				default:
				}
			}
		}
	}()
	stderrText := func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}

	waitErr := make(chan error, 1)
	go func() {
		<-done // the pipe must be drained before Wait closes it
		waitErr <- cmd.Wait()
	}()

	select {
	case u := <-authURL:
		if err := approveInBrowser(u); err != nil {
			// The CLI keeps waiting for a redirect that never comes.
			_ = cmd.Process.Kill()
			<-waitErr
			return stderrText(), &browserError{err}
		}
	case err := <-waitErr:
		// Failed before asking for the browser (e.g. the preflight).
		return stderrText(), err
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		<-waitErr
		t.Fatalf("login printed no authorize URL\nstderr:\n%s", stderrText())
	}

	select {
	case err := <-waitErr:
		return stderrText(), err
	case <-time.After(60 * time.Second):
		_ = cmd.Process.Kill()
		<-waitErr
		t.Fatalf("login did not finish\nstderr:\n%s", stderrText())
	}
	return "", nil
}

// browserError is a failure of the simulated browser, not of the CLI.
type browserError struct{ error }

// approveInBrowser does what a user does in the browser: log in to Redmine as
// the e2e admin, grant the consent form and follow the redirect to the CLI's
// loopback listener.
func approveInBrowser(authorizeURL string) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	base := e2eBaseURL()

	page, _, err := browserGet(client, base+"/login")
	if err != nil {
		return err
	}
	token, err := authenticityToken(page)
	if err != nil {
		return fmt.Errorf("login page: %w", err)
	}
	resp, err := client.PostForm(base+"/login", url.Values{
		"authenticity_token": {token},
		"username":           {e2eUsername()},
		"password":           {e2ePassword()},
	})
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		return fmt.Errorf("admin login: HTTP %d", resp.StatusCode)
	}

	body, resp, err := browserGet(client, authorizeURL)
	if err != nil {
		return err
	}
	location := resp.Header.Get("Location")
	if resp.StatusCode == http.StatusOK {
		form, err := consentForm(body)
		if err != nil {
			return err
		}
		resp, err = client.PostForm(base+"/oauth/authorize", form)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		location = resp.Header.Get("Location")
	}
	if !strings.HasPrefix(location, "http://127.0.0.1:") {
		return fmt.Errorf("consent did not redirect to the CLI (HTTP %d, Location %q)\n%s", resp.StatusCode, location, textOf(body))
	}

	// The CLI's listener is plain loopback HTTP; no cookies belong there.
	cb, err := http.Get(location)
	if err != nil {
		return fmt.Errorf("callback: %w", err)
	}
	_ = cb.Body.Close()
	return nil
}

// browserGet fetches rawURL and returns the body and response.
func browserGet(client *http.Client, rawURL string) (string, *http.Response, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return string(b), resp, err
}

// authenticityToken returns the first CSRF token in a Redmine page.
func authenticityToken(page string) (string, error) {
	inputs, err := formInputs(page, func(n *html.Node) bool { return hasInput(n, "authenticity_token", "") })
	if err != nil {
		return "", err
	}
	if tok := inputs.Get("authenticity_token"); tok != "" {
		return tok, nil
	}
	return "", errors.New("no authenticity_token in the page")
}

// consentForm returns the fields of Doorkeeper's authorize form (the one that
// posts to /oauth/authorize and is not the deny form).
func consentForm(page string) (url.Values, error) {
	form, err := formInputs(page, func(n *html.Node) bool {
		return strings.HasSuffix(attr(n, "action"), "/oauth/authorize") && !hasInput(n, "_method", "delete")
	})
	if err != nil {
		return nil, err
	}
	if form.Get("client_id") == "" {
		return nil, errors.New("no consent form in the page")
	}
	return form, nil
}

// hasInput reports whether form n holds an input with the given name and
// value (any value when empty).
func hasInput(n *html.Node, name, value string) bool {
	if n.Type == html.ElementNode && n.Data == "input" && attr(n, "name") == name && (value == "" || attr(n, "value") == value) {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hasInput(c, name, value) {
			return true
		}
	}
	return false
}

// formInputs collects the named inputs of the first <form> accepted by match.
func formInputs(page string, match func(*html.Node) bool) (url.Values, error) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil, err
	}
	var form *html.Node
	var find func(n *html.Node)
	find = func(n *html.Node) {
		if form != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "form" && match(n) {
			form = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)

	values := url.Values{}
	if form == nil {
		return values, nil
	}
	var collect func(n *html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" && attr(n, "name") != "" {
			values.Add(attr(n, "name"), attr(n, "value"))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(form)
	return values, nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// storedOAuth is the oauth block of the profile in the runner's config file.
type storedOAuth struct {
	ClientID     string    `yaml:"client_id"`
	ClientSecret string    `yaml:"client_secret"`
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	Expiry       time.Time `yaml:"expiry"`
	Scopes       []string  `yaml:"scopes"`
}

// readStoredOAuth reads the oauth block of the oauthProfile from the config.
func readStoredOAuth(t *testing.T, r *cliRunner) storedOAuth {
	t.Helper()
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg struct {
		Profiles map[string]struct {
			AuthMethod string      `yaml:"auth_method"`
			OAuth      storedOAuth `yaml:"oauth"`
		} `yaml:"profiles"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	p, ok := cfg.Profiles[oauthProfile]
	if !ok || p.AuthMethod != "oauth" {
		t.Fatalf("profile %q is not an oauth profile:\n%s", oauthProfile, data)
	}
	return p.OAuth
}

// expireStoredToken moves the stored access token's expiry into the past so
// the next command has to refresh.
func expireStoredToken(t *testing.T, r *cliRunner) {
	t.Helper()
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	profiles, _ := doc["profiles"].(map[string]any)
	profile, _ := profiles[oauthProfile].(map[string]any)
	oauthBlock, _ := profile["oauth"].(map[string]any)
	if oauthBlock == nil {
		t.Fatalf("no oauth block in config:\n%s", data)
	}
	oauthBlock["expiry"] = time.Now().Add(-time.Hour).UTC()
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(r.configPath, out, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// oauthEndpointPost posts form to Redmine's /oauth/<path> as app and returns
// the status and body.
func oauthEndpointPost(t *testing.T, app oauthApp, path string, form url.Values) (int, string) {
	t.Helper()
	form.Set("client_id", app.ID)
	form.Set("client_secret", app.Secret)
	resp, err := http.PostForm(e2eBaseURL()+"/oauth/"+path, form)
	if err != nil {
		t.Fatalf("POST /oauth/%s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// bearerStatus returns the HTTP status of /users/current.json with token.
func bearerStatus(t *testing.T, token string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e2eBaseURL()+"/users/current.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /users/current.json: %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// textOf strips markup from a page for error messages.
func textOf(page string) string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return page
	}
	var sb strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			if s := strings.TrimSpace(n.Data); s != "" {
				sb.WriteString(s + " ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return sb.String()
}
