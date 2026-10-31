package oauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

// LoginTimeout bounds the browser login. A variable so tests can shorten it.
var LoginTimeout = 5 * time.Minute

// CallbackPath is the redirect path; admins register http://127.0.0.1/callback.
const CallbackPath = "/callback"

// listen opens the loopback listener. A variable so tests can replace it.
var listen = net.Listen

const (
	successPage = "Logged in to Redmine. You can close this tab."
	failurePage = "Login failed. Return to the terminal."
)

// Flow is one browser login: a loopback listener, PKCE verifier and state.
type Flow struct {
	c           *Client
	scopes      []string
	verifier    string
	state       string
	redirectURI string
	authURL     string
	host        string

	ln        net.Listener
	srv       *http.Server
	result    chan callbackResult // first result wins
	closeOnce sync.Once
	closeErr  error
}

// callbackResult is what the redirect carried: an authorization code or an
// error.
type callbackResult struct {
	code string
	err  error
}

// Start listens on 127.0.0.1:0 and builds the authorize URL (response_type
// code, client_id, redirect_uri http://127.0.0.1:<port>/callback, scope,
// state, code_challenge S256).
func (c *Client) Start(scopes []string) (*Flow, error) {
	ln, err := listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listening for the login redirect: %w", err)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("generating the login state: %w", err)
	}
	f := &Flow{
		c:        c,
		scopes:   scopes,
		verifier: oauth2.GenerateVerifier(),
		state:    base64.RawURLEncoding.EncodeToString(buf),
		ln:       ln,
		result:   make(chan callbackResult, 1),
	}
	f.host = ln.Addr().String()
	f.redirectURI = "http://" + f.host + CallbackPath
	f.authURL = c.oauth2Config(f.redirectURI, scopes).AuthCodeURL(f.state, oauth2.S256ChallengeOption(f.verifier))

	f.srv = &http.Server{Handler: http.HandlerFunc(f.handle), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = f.srv.Serve(ln) }()
	c.Log.Printf("OAuth: waiting for the login redirect on %s", f.redirectURI)
	return f, nil
}

// URL returns the authorize URL to open in a browser.
func (f *Flow) URL() string { return f.authURL }

// Deliver accepts the URL the browser ended on, pasted by the user when the
// browser cannot reach this machine (SSH). It returns an error for a URL that
// is not this flow's redirect (wrong state, no code) so the prompt can ask
// again; a Redmine error (error=...) is delivered as the flow result.
func (f *Flow) Deliver(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.RawQuery == "" {
		return errors.New("not a login redirect URL")
	}
	return f.accept(u.Query())
}

// Wait waits for the redirect (listener or Deliver), exchanges the code and
// returns the token. ctx bounds the wait.
func (f *Flow) Wait(ctx context.Context) (Token, error) {
	select {
	case <-ctx.Done():
		return Token{}, ctx.Err()
	case res := <-f.result:
		if res.err != nil {
			return Token{}, res.err
		}
		return f.c.exchange(ctx, res.code, f.verifier, f.redirectURI, f.scopes)
	}
}

// Close shuts the listener down. It is safe to call more than once.
func (f *Flow) Close() error {
	f.closeOnce.Do(func() {
		f.closeErr = f.srv.Close()
		// Serve may not have started yet, so close the listener directly too.
		_ = f.ln.Close()
	})
	return f.closeErr
}

// accept validates the redirect's query and records the first result. A wrong
// state or a missing code is returned as an error and records nothing.
func (f *Flow) accept(q url.Values) error {
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(f.state)) != 1 {
		return errors.New("the login state does not match this login")
	}
	var res callbackResult
	switch errCode := q.Get("error"); {
	case errCode == "access_denied":
		res.err = ErrAccessDenied
	case errCode == "invalid_scope":
		res.err = ErrInvalidScope
	case errCode != "":
		res.err = fmt.Errorf("Redmine returned %s", textsafe.Terminal(errCode)+describeParam(q.Get("error_description")))
	case q.Get("code") == "":
		return errors.New("the URL has no authorization code")
	default:
		res.code = q.Get("code")
	}
	select {
	case f.result <- res:
	default: // a result is already waiting
	}
	return nil
}

func describeParam(desc string) string {
	if desc == "" {
		return ""
	}
	return ": " + textsafe.Terminal(desc)
}

func (f *Flow) handle(w http.ResponseWriter, r *http.Request) {
	if r.Host != f.host {
		http.Error(w, "bad host", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodGet || r.URL.Path != CallbackPath {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	err := f.accept(q)
	failed := err != nil || q.Get("error") != ""
	page := successPage
	status := http.StatusOK
	if failed {
		page = failurePage
	}
	if err != nil {
		status = http.StatusBadRequest
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(page))
}
