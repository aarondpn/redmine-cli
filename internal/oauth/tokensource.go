package oauth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// refreshTimeout bounds the token request made while the config lock is held,
// so a stalled Redmine cannot block other redmine processes for long.
var refreshTimeout = 20 * time.Second

// refreshSkew is how long before expiry an access token is refreshed.
const refreshSkew = time.Minute

// Credentials identify the OAuth application.
type Credentials struct {
	ClientID, ClientSecret string
}

// ErrStoreChanged is returned by Store.Save when the stored profile is gone
// or no longer the one the tokens belong to.
var ErrStoreChanged = errors.New("the stored profile changed")

// Store keeps a profile's tokens across processes.
type Store interface {
	Lock(ctx context.Context) (unlock func(), err error)
	// Load returns the stored tokens and the application they belong to.
	Load() (Token, Credentials, error)
	Save(Token) error
}

// TokenSource hands out a valid access token, refreshing it when it is about
// to expire or Redmine rejected it. Refresh tokens rotate, so refreshes are
// serialized: in-process by mu, across processes by Store.Lock, and the
// stored tokens are re-read under the lock before refreshing.
type TokenSource struct {
	c     *Client
	store Store // nil: memory only
	log   *debug.Logger

	mu sync.Mutex // serializes refreshes; guards c and unsaved
	// unsaved is the stored refresh token that refreshes could not replace
	// because saving failed; while the store still holds it, our token is
	// newer.
	unsaved string
	tmu     sync.RWMutex
	token   Token // guarded by tmu
}

// NewTokenSource returns a TokenSource starting from initial. store may be
// nil, then tokens live in memory only.
func NewTokenSource(c *Client, initial Token, store Store, log *debug.Logger) *TokenSource {
	return &TokenSource{c: c, token: initial, store: store, log: log}
}

// Current returns the current token pair.
func (s *TokenSource) Current() Token {
	s.tmu.RLock()
	defer s.tmu.RUnlock()
	return s.token
}

func (s *TokenSource) set(t Token) {
	s.tmu.Lock()
	s.token = t
	s.tmu.Unlock()
}

// fresh reports whether the access token is usable for a while. A zero
// expiry counts as fresh. Round(0) drops the monotonic reading, which stops
// while the machine sleeps.
func fresh(t Token) bool {
	return t.AccessToken != "" && (t.Expiry.IsZero() || time.Until(t.Expiry.Round(0)) > refreshSkew)
}

// Token returns an access token, refreshing it first when it is about to
// expire.
func (s *TokenSource) Token(ctx context.Context) (string, error) {
	cur := s.Current()
	if fresh(cur) {
		return cur.AccessToken, nil
	}
	return s.Refresh(ctx, cur.AccessToken)
}

// Refresh returns a usable access token after Redmine rejected (or the clock
// expired) the access token rejected. It may return a token another goroutine
// or process refreshed in the meantime.
func (s *TokenSource) Refresh(ctx context.Context, rejected string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Another goroutine already refreshed.
	if cur := s.Current(); cur.AccessToken != rejected && fresh(cur) {
		return cur.AccessToken, nil
	}

	if s.store != nil {
		unlock, err := s.store.Lock(ctx)
		if err != nil {
			return "", err
		}
		defer unlock()
	}

	base := s.Current()
	var storedRefresh string
	if s.store != nil {
		stored, creds, err := s.store.Load()
		if err != nil {
			return "", fmt.Errorf("reading the stored OAuth token: %w", err)
		}
		// A login with another application or secret since this process
		// started: refresh with the stored credentials.
		if creds.ClientID != "" && (creds.ClientID != s.c.ClientID || creds.ClientSecret != s.c.ClientSecret) {
			c := *s.c
			c.ClientID, c.ClientSecret = creds.ClientID, creds.ClientSecret
			s.c = &c
		}
		storedRefresh = stored.RefreshToken
		// The store still holds the refresh token our refreshes could not
		// replace (it could not be written): ours is newer.
		if s.unsaved == "" || stored.RefreshToken != s.unsaved {
			// Another process refreshed, or the user logged in again.
			if stored.RefreshToken != "" && stored.AccessToken != rejected && fresh(stored) {
				s.unsaved = ""
				s.set(stored)
				return stored.AccessToken, nil
			}
			s.unsaved = ""
			base = stored
		}
	}
	if base.RefreshToken == "" {
		return "", ErrLoginRequired
	}

	rctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	next, err := s.c.Refresh(rctx, base.RefreshToken)
	if err != nil {
		return "", err
	}
	if len(next.Scopes) == 0 {
		next.Scopes = base.Scopes
	}
	if s.store != nil {
		// A store that cannot be written (read-only config) must not make
		// every request refresh again: the old refresh token is spent once
		// the new access token is used, so keep the pair in memory.
		if err := s.store.Save(next); err != nil {
			if errors.Is(err, ErrStoreChanged) {
				return "", fmt.Errorf("saving the refreshed OAuth token: %w", err)
			}
			s.unsaved = storedRefresh
			s.log.Printf("OAuth: could not save the refreshed token, keeping it in memory: %v", err)
		} else {
			s.unsaved = ""
		}
	}
	s.set(next)
	s.log.Printf("OAuth: refreshed access token (expires %s)", expiryText(next.Expiry))
	return next.AccessToken, nil
}
