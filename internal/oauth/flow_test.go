package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/oauth/oauthtest"
)

func startFlow(t *testing.T, srv *oauthtest.Server, scopes ...string) *Flow {
	t.Helper()
	if len(scopes) == 0 {
		scopes = []string{"view_issues"}
	}
	f, err := newClient(srv).Start(scopes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// callback GETs the flow's redirect URI with extra query parameters.
func callback(t *testing.T, f *Flow, query url.Values) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(f.redirectURI + "?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func waitCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestFlowURL(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f := startFlow(t, srv, "view_issues", "log_time")

	u, err := url.Parse(f.URL())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := u.Scheme+"://"+u.Host+u.Path, srv.URL+"/oauth/authorize"; got != want {
		t.Errorf("authorize endpoint = %s, want %s", got, want)
	}
	q := u.Query()
	sum := sha256.Sum256([]byte(f.verifier))
	_, port, _ := net.SplitHostPort(f.ln.Addr().String())
	want := map[string]string{
		"response_type":         "code",
		"client_id":             srv.ClientID,
		"redirect_uri":          "http://127.0.0.1:" + port + "/callback",
		"scope":                 "view_issues log_time",
		"state":                 f.state,
		"code_challenge":        base64.RawURLEncoding.EncodeToString(sum[:]),
		"code_challenge_method": "S256",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if len(f.state) < 40 {
		t.Errorf("state %q is too short", f.state)
	}
}

func TestFlowApproveWait(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f := startFlow(t, srv)
	srv.Approve(t, f.URL())
	tok, err := f.Wait(waitCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Errorf("token = %+v", tok)
	}
}

func TestFlowCallbackPage(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f := startFlow(t, srv)
	resp, body := callback(t, f, url.Values{"code": {"c"}, "state": {f.state}})
	if resp.StatusCode != http.StatusOK || body != "Logged in to Redmine. You can close this tab." {
		t.Errorf("success page = %d %q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
}

func TestFlowWrongStateIgnored(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f := startFlow(t, srv)

	resp, body := callback(t, f, url.Values{"code": {"evil"}, "state": {"wrong"}})
	if resp.StatusCode != http.StatusBadRequest || body != "Login failed. Return to the terminal." {
		t.Errorf("wrong state answer = %d %q", resp.StatusCode, body)
	}
	select {
	case res := <-f.result:
		t.Fatalf("wrong state was accepted: %+v", res)
	default:
	}

	srv.Approve(t, f.URL())
	if _, err := f.Wait(waitCtx(t)); err != nil {
		t.Fatalf("Wait() after a wrong state = %v", err)
	}
}

func TestFlowRedirectErrors(t *testing.T) {
	tests := []struct {
		name  string
		query url.Values
		want  error
		text  string
	}{
		{"access_denied", url.Values{"error": {"access_denied"}}, ErrAccessDenied, ""},
		{"invalid_scope", url.Values{"error": {"invalid_scope"}}, ErrInvalidScope, ""},
		{"other", url.Values{"error": {"server_error"}, "error_description": {"boom\x1b[2J"}}, nil, "Redmine returned server_error: boom^[[2J"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := startFlow(t, oauthtest.NewServer(t))
			tt.query.Set("state", f.state)
			resp, body := callback(t, f, tt.query)
			if resp.StatusCode != http.StatusOK || body != "Login failed. Return to the terminal." {
				t.Errorf("error page = %d %q", resp.StatusCode, body)
			}
			_, err := f.Wait(waitCtx(t))
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Wait() = %v, want %v", err, tt.want)
			}
			if tt.text != "" && (err == nil || err.Error() != tt.text) {
				t.Errorf("Wait() = %v, want %q", err, tt.text)
			}
		})
	}
}

func TestFlowFirstResultWins(t *testing.T) {
	f := startFlow(t, oauthtest.NewServer(t))
	callback(t, f, url.Values{"error": {"access_denied"}, "state": {f.state}})
	callback(t, f, url.Values{"code": {"late"}, "state": {f.state}})
	if _, err := f.Wait(waitCtx(t)); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("Wait() = %v, want ErrAccessDenied", err)
	}
}

func TestFlowDeliver(t *testing.T) {
	srv := oauthtest.NewServer(t)
	f := startFlow(t, srv)

	for _, in := range []string{
		"not a url",
		"http://127.0.0.1:1/callback",
		"http://127.0.0.1:1/callback?code=abc&state=wrong",
		"http://127.0.0.1:1/callback?code=abc",
		"http://127.0.0.1:1/callback?state=" + f.state,
	} {
		if err := f.Deliver(in); err == nil {
			t.Errorf("Deliver(%q) = nil, want an error", in)
		}
	}

	// Still waiting after the bad pastes; the real redirect completes the login.
	select {
	case res := <-f.result:
		t.Fatalf("a bad paste produced a result: %+v", res)
	default:
	}
	srv.Approve(t, f.URL())
	if _, err := f.Wait(waitCtx(t)); err != nil {
		t.Fatal(err)
	}
}

func TestFlowDeliverValid(t *testing.T) {
	srv := oauthtest.NewServer(t)
	// The paste path: the fake browser's redirect never reaches the listener,
	// so the code is taken from a redirect captured by a stand-in server.
	f := startFlow(t, srv)
	redirected := make(chan string, 1)
	catcher := newCatcher(t, redirected)
	authURL, _ := url.Parse(f.URL())
	q := authURL.Query()
	q.Set("redirect_uri", catcher+"/callback")
	authURL.RawQuery = q.Encode()
	srv.Approve(t, authURL.String())

	pasted := <-redirected
	if err := f.Deliver("  " + pasted + "\n"); err != nil {
		t.Fatalf("Deliver(%q) = %v", pasted, err)
	}
	// The code was minted for the catcher's port; the server ignores ports for
	// loopback redirects, so the exchange succeeds.
	tok, err := f.Wait(waitCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" {
		t.Errorf("token = %+v", tok)
	}
}

func TestFlowDeliverError(t *testing.T) {
	f := startFlow(t, oauthtest.NewServer(t))
	if err := f.Deliver("http://127.0.0.1:1/callback?error=access_denied&state=" + f.state); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Wait(waitCtx(t)); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("Wait() = %v, want ErrAccessDenied", err)
	}
}

func TestFlowHostGuard(t *testing.T) {
	f := startFlow(t, oauthtest.NewServer(t))
	req, _ := http.NewRequest(http.MethodGet, f.redirectURI+"?code=c&state="+f.state, nil)
	req.Host = "evil.example"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	select {
	case res := <-f.result:
		t.Fatalf("a request with a foreign Host was accepted: %+v", res)
	default:
	}
}

func TestFlowRouting(t *testing.T) {
	f := startFlow(t, oauthtest.NewServer(t))
	resp, err := http.Get("http://" + f.host + "/other?code=c&state=" + f.state)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /other = %d, want 404", resp.StatusCode)
	}
	resp, err = http.Post(f.redirectURI+"?code=c&state="+f.state, "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("POST /callback = %d, want 404", resp.StatusCode)
	}
	select {
	case res := <-f.result:
		t.Fatalf("unexpected result: %+v", res)
	default:
	}
}

func TestFlowWaitContext(t *testing.T) {
	f := startFlow(t, oauthtest.NewServer(t))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := f.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Wait() = %v, want DeadlineExceeded", err)
	}
}

func TestFlowClose(t *testing.T) {
	f := startFlow(t, oauthtest.NewServer(t))
	addr := f.ln.Addr().String()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port %s not freed: %v", addr, err)
	}
	_ = ln.Close()
}

func TestStartListenError(t *testing.T) {
	old := listen
	listen = func(network, address string) (net.Listener, error) { return nil, errors.New("no sockets") }
	defer func() { listen = old }()
	if _, err := newClient(oauthtest.NewServer(t)).Start(nil); err == nil || !strings.Contains(err.Error(), "no sockets") {
		t.Errorf("Start() = %v", err)
	}
}

// newCatcher starts a loopback server that reports the full URL of the first
// request it gets, like a browser address bar after the redirect.
func newCatcher(t *testing.T, got chan<- string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case got <- "http://" + r.Host + r.URL.RequestURI():
		default:
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String()
}
