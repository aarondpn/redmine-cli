package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/filelock"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth/oauthtest"
)

// memStore is an in-process Store.
type memStore struct {
	mu      sync.Mutex // the "cross-process" lock
	tok     Token
	creds   Credentials // zero: the store does not know the application
	loads   int
	saves   int
	saveErr error
	loadErr error
}

func (m *memStore) Lock(ctx context.Context) (func(), error) {
	m.mu.Lock()
	return m.mu.Unlock, nil
}

func (m *memStore) Load() (Token, Credentials, error) {
	m.loads++
	if m.loadErr != nil {
		return Token{}, Credentials{}, m.loadErr
	}
	return m.tok, m.creds, nil
}

func (m *memStore) Save(t Token) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saves++
	m.tok = t
	return nil
}

// issued returns a token as a login would have stored it.
func issued(srv *oauthtest.Server, expiry time.Time) Token {
	access, refresh := srv.Issue([]string{"view_issues"})
	return Token{AccessToken: access, RefreshToken: refresh, Expiry: expiry, Scopes: []string{"view_issues"}}
}

func TestTokenSourceFresh(t *testing.T) {
	srv := oauthtest.NewServer(t)
	for _, expiry := range []time.Time{{}, time.Now().Add(time.Hour)} {
		tok := issued(srv, expiry)
		store := &memStore{tok: tok}
		ts := NewTokenSource(newClient(srv), tok, store, nil)
		got, err := ts.Token(context.Background())
		if err != nil || got != tok.AccessToken {
			t.Fatalf("Token() = %q, %v", got, err)
		}
	}
	if srv.Refreshes.Load() != 0 {
		t.Errorf("Refreshes = %d, want 0", srv.Refreshes.Load())
	}
}

func TestTokenSourceNearExpiryRefreshes(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(30*time.Second))
	store := &memStore{tok: tok}
	ts := NewTokenSource(newClient(srv), tok, store, nil)

	got, err := ts.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == tok.AccessToken || got != ts.Current().AccessToken {
		t.Errorf("Token() = %q, old %q", got, tok.AccessToken)
	}
	if srv.Refreshes.Load() != 1 || store.saves != 1 || store.tok.AccessToken != got {
		t.Errorf("refreshes=%d saves=%d stored=%q", srv.Refreshes.Load(), store.saves, store.tok.AccessToken)
	}
	if len(store.tok.Scopes) == 0 || store.tok.RefreshToken == tok.RefreshToken {
		t.Errorf("stored token = %+v", store.tok)
	}
}

func TestTokenSourceNilStore(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	ts := NewTokenSource(newClient(srv), tok, nil, nil)
	got, err := ts.Token(context.Background())
	if err != nil || got == tok.AccessToken {
		t.Fatalf("Token() = %q, %v", got, err)
	}
}

func TestTokenSourceAdoptsNewerStoredToken(t *testing.T) {
	srv := oauthtest.NewServer(t)
	old := issued(srv, time.Now().Add(-time.Minute))
	newer := issued(srv, time.Now().Add(time.Hour))
	store := &memStore{tok: newer}
	ts := NewTokenSource(newClient(srv), old, store, nil)

	got, err := ts.Token(context.Background())
	if err != nil || got != newer.AccessToken {
		t.Fatalf("Token() = %q, %v, want the stored token", got, err)
	}
	if srv.Refreshes.Load() != 0 || store.saves != 0 {
		t.Errorf("refreshes=%d saves=%d, want none", srv.Refreshes.Load(), store.saves)
	}
}

func TestTokenSourceRejectedToken(t *testing.T) {
	srv := oauthtest.NewServer(t)
	// Not expired by the clock, but Redmine said 401.
	tok := issued(srv, time.Now().Add(time.Hour))
	store := &memStore{tok: tok}
	ts := NewTokenSource(newClient(srv), tok, store, nil)

	got, err := ts.Refresh(context.Background(), tok.AccessToken)
	if err != nil || got == tok.AccessToken {
		t.Fatalf("Refresh() = %q, %v", got, err)
	}
	if srv.Refreshes.Load() != 1 {
		t.Errorf("Refreshes = %d, want 1", srv.Refreshes.Load())
	}
	// A second 401 for the already replaced token reuses the new one.
	again, err := ts.Refresh(context.Background(), tok.AccessToken)
	if err != nil || again != got || srv.Refreshes.Load() != 1 {
		t.Errorf("Refresh(stale) = %q, %v, refreshes=%d", again, err, srv.Refreshes.Load())
	}
}

func TestTokenSourceLoginRequiredThenRecovers(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	store := &memStore{tok: tok}
	ts := NewTokenSource(newClient(srv), tok, store, nil)

	if err := newClient(srv).Revoke(context.Background(), tok.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Token(context.Background()); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Token() = %v, want ErrLoginRequired", err)
	}

	// The user runs "redmine auth login": the store gets a new token.
	store.tok = issued(srv, time.Now().Add(time.Hour))
	got, err := ts.Token(context.Background())
	if err != nil || got != store.tok.AccessToken {
		t.Fatalf("Token() after login = %q, %v", got, err)
	}
}

func TestTokenSourceStoreWithoutRefreshToken(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	ts := NewTokenSource(newClient(srv), tok, &memStore{}, nil)
	if _, err := ts.Token(context.Background()); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Token() = %v, want ErrLoginRequired", err)
	}
}

func TestTokenSourceStoreReadErrorIsNotLoginRequired(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	ts := NewTokenSource(newClient(srv), tok, &memStore{loadErr: errors.New("keyring locked")}, nil)
	_, err := ts.Token(context.Background())
	if err == nil || errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Token() = %v, want a store read error", err)
	}
}

func TestTokenSourceSaveFailureKeepsNewToken(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	store := &memStore{tok: tok, saveErr: errors.New("permission denied")}
	ts := NewTokenSource(newClient(srv), tok, store, nil)

	// A config that cannot be written must not refresh on every request.
	var first string
	for i := 0; i < 3; i++ {
		got, err := ts.Token(context.Background())
		if err != nil {
			t.Fatalf("Token() = %v", err)
		}
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("Token() = %q, want the kept %q", got, first)
		}
	}
	if srv.Refreshes.Load() != 1 {
		t.Errorf("Refreshes = %d, want 1", srv.Refreshes.Load())
	}
}

func TestTokenSourceChangedProfileIsAnError(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	store := &memStore{tok: tok, saveErr: ErrStoreChanged}
	ts := NewTokenSource(newClient(srv), tok, store, nil)

	if _, err := ts.Token(context.Background()); !errors.Is(err, ErrStoreChanged) {
		t.Fatalf("Token() = %v, want ErrStoreChanged", err)
	}
	if ts.Current().AccessToken != tok.AccessToken {
		t.Error("the new token was adopted although the profile changed")
	}
}

func TestTokenSourceUsesStoredClientCredentials(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	store := &memStore{tok: tok, creds: Credentials{ClientID: srv.ClientID, ClientSecret: srv.ClientSecret}}
	// The process started with a secret that was rotated since.
	stale := newClient(srv)
	stale.ClientSecret = "old-secret"
	ts := NewTokenSource(stale, tok, store, nil)

	if _, err := ts.Token(context.Background()); err != nil {
		t.Fatalf("Token() = %v, want a refresh with the stored secret", err)
	}
	if srv.Refreshes.Load() != 1 {
		t.Errorf("Refreshes = %d, want 1", srv.Refreshes.Load())
	}
	if stale.ClientSecret != "old-secret" {
		t.Error("the caller's client was modified")
	}
}

func TestTokenSourceConcurrentRefresh(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	ts := NewTokenSource(newClient(srv), tok, &memStore{tok: tok}, nil)

	var wg sync.WaitGroup
	results := make([]string, 20)
	errs := make([]error, 20)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = ts.Token(context.Background())
		}()
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i] != results[0] {
			t.Fatalf("goroutine %d: %q, %v (first %q)", i, results[i], errs[i], results[0])
		}
	}
	if srv.Refreshes.Load() != 1 {
		t.Errorf("Refreshes = %d, want 1", srv.Refreshes.Load())
	}
}

// fileStore keeps a token in a JSON file guarded by a filelock, like the
// config file shared by CLI processes.
type fileStore struct{ path string }

func (f fileStore) Lock(ctx context.Context) (func(), error) {
	l, err := filelock.Acquire(ctx, f.path+".lock")
	if err != nil {
		return nil, err
	}
	return func() { _ = l.Unlock() }, nil
}

func (f fileStore) Load() (Token, Credentials, error) {
	var t Token
	data, err := os.ReadFile(f.path)
	if err != nil {
		return t, Credentials{}, err
	}
	return t, Credentials{}, json.Unmarshal(data, &t)
}

func (f fileStore) Save(t Token) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o600)
}

func TestTokenSourceTwoProcessesShareOneRefresh(t *testing.T) {
	srv := oauthtest.NewServer(t)
	tok := issued(srv, time.Now().Add(-time.Minute))
	store := fileStore{path: filepath.Join(t.TempDir(), "tokens.json")}
	if err := store.Save(tok); err != nil {
		t.Fatal(err)
	}
	// Separate in-memory state, like two CLI processes.
	a := NewTokenSource(newClient(srv), tok, store, nil)
	b := NewTokenSource(newClient(srv), tok, store, nil)

	var wg sync.WaitGroup
	got := make([]string, 2)
	errs := make([]error, 2)
	for i, ts := range []*TokenSource{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = ts.Token(context.Background())
		}()
	}
	wg.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("source %d: %v (invalid_grant means a refresh token was reused)", i, errs[i])
		}
	}
	if got[0] != got[1] {
		t.Errorf("tokens differ: %q vs %q", got[0], got[1])
	}
	if srv.Refreshes.Load() != 1 {
		t.Errorf("Refreshes = %d, want 1", srv.Refreshes.Load())
	}

	// The shared token works against Redmine.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/users/current.json", nil)
	req.Header.Set("Authorization", "Bearer "+got[0])
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestTokenSourceUsesKeptTokenWhenSaveFailed(t *testing.T) {
	srv := oauthtest.NewServer(t)
	srv.ExpiresIn = 30
	srv.Mux.Handle("/probe", srv.RequireBearer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	tok := issued(srv, time.Now().Add(-time.Minute))
	store := &memStore{tok: tok, saveErr: errors.New("read-only file system")}
	ts := NewTokenSource(newClient(srv), tok, store, nil)

	first, err := ts.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Using the new token makes Redmine revoke the old grant.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/probe", nil)
	req.Header.Set("Authorization", "Bearer "+first)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("probe = %v, %v", resp, err)
	}
	resp.Body.Close()

	// The store still holds the spent pair; the kept one must be refreshed.
	if _, err := ts.Token(context.Background()); err != nil {
		t.Fatalf("second Token() = %v", err)
	}
}

func TestTokenSourceKeepsRefreshingWhenSaveKeepsFailing(t *testing.T) {
	srv := oauthtest.NewServer(t)
	srv.ExpiresIn = 30 // always inside refreshSkew
	srv.Mux.Handle("/probe", srv.RequireBearer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	tok := issued(srv, time.Now().Add(-time.Minute))
	store := &memStore{tok: tok, saveErr: errors.New("read-only file system")}
	ts := NewTokenSource(newClient(srv), tok, store, nil)

	for i := 1; i <= 5; i++ {
		access, err := ts.Token(context.Background())
		if err != nil {
			t.Fatalf("Token() #%d = %v", i, err)
		}
		// Using the new token makes Redmine revoke the previous grant.
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/probe", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("probe #%d = %v, %v", i, resp, err)
		}
		_ = resp.Body.Close()
	}
}

func TestTokenSourceRefreshTimesOut(t *testing.T) {
	orig := refreshTimeout
	refreshTimeout = 50 * time.Millisecond
	t.Cleanup(func() { refreshTimeout = orig })

	release := make(chan struct{})
	stalled := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer stalled.Close()
	defer close(release)

	tok := Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(-time.Minute)}
	store := &memStore{tok: tok}
	ts := NewTokenSource(&Client{Server: stalled.URL, ClientID: "a", ClientSecret: "b", HTTPClient: stalled.Client()}, tok, store, nil)
	start := time.Now()
	if _, err := ts.Token(context.Background()); err == nil {
		t.Fatal("Token() succeeded against a stalled server")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("refresh did not time out")
	}
}

func TestFreshIgnoresMonotonicClock(t *testing.T) {
	if !fresh(Token{AccessToken: "a", Expiry: time.Now().Add(time.Hour)}) {
		t.Error("token expiring in an hour is not fresh")
	}
	if fresh(Token{AccessToken: "a", Expiry: time.Now().Add(-time.Second)}) {
		t.Error("expired token is fresh")
	}
}
