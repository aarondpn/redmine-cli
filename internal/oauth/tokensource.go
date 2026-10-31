package oauth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// refreshSkew is how long before expiry an access token is refreshed.
const refreshSkew = time.Minute

// Store keeps a profile's tokens across processes.
type Store interface {
	Lock(ctx context.Context) (unlock func(), err error)
	Load() (Token, error)
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

	mu    sync.Mutex // serializes refreshes
	tmu   sync.RWMutex
	token Token // guarded by tmu
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
// expiry counts as fresh.
func fresh(t Token) bool {
	return t.AccessToken != "" && (t.Expiry.IsZero() || time.Until(t.Expiry) > refreshSkew)
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
	if s.store != nil {
		stored, err := s.store.Load()
		if err != nil {
			return "", fmt.Errorf("reading the stored OAuth token: %w", err)
		}
		// Another process refreshed, or the user logged in again.
		if stored.RefreshToken != "" && stored.AccessToken != rejected && fresh(stored) {
			s.set(stored)
			return stored.AccessToken, nil
		}
		base = stored
	}
	if base.RefreshToken == "" {
		return "", ErrLoginRequired
	}

	next, err := s.c.Refresh(ctx, base.RefreshToken)
	if err != nil {
		return "", err
	}
	if len(next.Scopes) == 0 {
		next.Scopes = base.Scopes
	}
	if s.store != nil {
		// The old refresh token stays valid until the new access token is
		// used, so not adopting next loses nothing.
		if err := s.store.Save(next); err != nil {
			return "", fmt.Errorf("saving the refreshed OAuth token: %w", err)
		}
	}
	s.set(next)
	s.log.Printf("OAuth: refreshed access token (expires %s)", expiryText(next.Expiry))
	return next.AccessToken, nil
}
