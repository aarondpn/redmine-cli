package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth"
)

// NewOAuthClient returns the client for cfg's OAuth endpoints. Its requests
// carry the configured headers and User-Agent and use the same guarded
// transport as API calls, but no Redmine credentials.
func NewOAuthClient(cfg *config.Config, log *debug.Logger) (*oauth.Client, error) {
	headerTimeout, err := responseHeaderTimeout()
	if err != nil {
		return nil, err
	}
	withScheme := *cfg
	withScheme.Server = config.EnsureScheme(cfg.Server)
	return newOAuthClient(&withScheme, log, newBaseTransport(headerTimeout)), nil
}

func newOAuthClient(cfg *config.Config, log *debug.Logger, base *http.Transport) *oauth.Client {
	server := strings.TrimRight(cfg.Server, "/")
	serverURL, err := url.Parse(server)
	if err != nil {
		serverURL = &url.URL{}
	}
	httpClient, _ := newGuardedClient(base, serverURL, func(guarded http.RoundTripper) http.RoundTripper {
		serverOrigin := config.OriginOf(serverURL)
		return &authTransport{
			base:       guarded,
			authMethod: authMethodNone,
			headers:    cfg.Headers,
			origin:     &serverOrigin,
		}
	})
	// The client secret and refresh token are already in these requests
	// when the transport sees them, so a redirect would carry them along.
	// Redmine answers the token endpoints directly.
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &oauth.Client{
		Server:       server,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
		HTTPClient:   httpClient,
		Log:          log,
	}
}

// newTokenSource starts from the tokens in cfg. Refreshed tokens are written
// back to the profile cfg was loaded from; without one they stay in memory.
func newTokenSource(cfg *config.Config, log *debug.Logger, base *http.Transport) *oauth.TokenSource {
	var store oauth.Store
	if path, profile, ok := cfg.ProfileLocation(); ok {
		store = &profileTokenStore{configPath: path, profile: profile, clientID: cfg.OAuth.ClientID}
	}
	initial := oauth.Token{
		AccessToken:  cfg.OAuth.AccessToken,
		RefreshToken: cfg.OAuth.RefreshToken,
		Expiry:       cfg.OAuth.Expiry,
		Scopes:       cfg.OAuth.Scopes,
	}
	return oauth.NewTokenSource(newOAuthClient(cfg, log, base), initial, store, log)
}

// profileTokenStore persists refreshed tokens to the profile they came from.
type profileTokenStore struct {
	configPath, profile string
	// clientID is the application the tokens belong to (as last loaded);
	// SaveOAuthTokens refuses to write them over a profile that logged in
	// with another.
	clientID string
}

func (s *profileTokenStore) Lock(ctx context.Context) (func(), error) {
	return config.LockConfig(ctx, s.configPath)
}

func (s *profileTokenStore) Load() (oauth.Token, oauth.Credentials, error) {
	o, err := config.LoadOAuthTokens(s.configPath, s.profile)
	if err != nil {
		return oauth.Token{}, oauth.Credentials{}, err
	}
	s.clientID = o.ClientID // Save writes for the application Load saw
	tok := oauth.Token{AccessToken: o.AccessToken, RefreshToken: o.RefreshToken, Expiry: o.Expiry, Scopes: o.Scopes}
	return tok, oauth.Credentials{ClientID: o.ClientID, ClientSecret: o.ClientSecret}, nil
}

func (s *profileTokenStore) Save(t oauth.Token) error {
	err := config.SaveOAuthTokens(s.configPath, s.profile, config.OAuthConfig{
		ClientID:     s.clientID,
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
		Scopes:       t.Scopes,
	})
	if errors.Is(err, config.ErrProfileChanged) {
		return fmt.Errorf("%w: %w", oauth.ErrStoreChanged, err)
	}
	return err
}

// OAuthToken returns the client's current OAuth token, refreshed if a
// request needed it. ok is false for other auth methods.
func (c *Client) OAuthToken() (oauth.Token, bool) {
	if c.oauthTokens == nil {
		return oauth.Token{}, false
	}
	return c.oauthTokens.Current(), true
}
