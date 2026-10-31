package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// preflightRedirect is the redirect URI an admin registers; Doorkeeper
// ignores the port of a loopback redirect.
const preflightRedirect = "http://127.0.0.1" + CallbackPath

// Client talks to Redmine's OAuth endpoints for one registered application.
type Client struct {
	Server       string // Redmine base URL, incl. sub-path
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client // sends no Redmine credentials
	Log          *debug.Logger
}

// Token is an OAuth token pair as Redmine issued it.
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time // zero: does not expire
	Scopes       []string
}

func (c *Client) base() string {
	return strings.TrimRight(c.Server, "/")
}

func (c *Client) oauth2Config(redirectURI string, scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   c.base() + "/oauth/authorize",
			TokenURL:  c.base() + "/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader,
		},
		RedirectURL: redirectURI,
		Scopes:      scopes,
	}
}

// httpContext routes oauth2's requests through c.HTTPClient.
func (c *Client) httpContext(ctx context.Context) context.Context {
	if c.HTTPClient == nil {
		return ctx
	}
	return context.WithValue(ctx, oauth2.HTTPClient, c.HTTPClient)
}

// toToken converts an oauth2 token. Scopes come from the "scope" field of the
// response and fall back to the given ones.
func toToken(tok *oauth2.Token, fallback []string) Token {
	scopes := fallback
	if s, ok := tok.Extra("scope").(string); ok && strings.TrimSpace(s) != "" {
		scopes = strings.Fields(s)
	}
	return Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
		Scopes:       scopes,
	}
}

// exchange trades an authorization code for tokens.
func (c *Client) exchange(ctx context.Context, code, verifier, redirectURI string, scopes []string) (Token, error) {
	cfg := c.oauth2Config(redirectURI, scopes)
	tok, err := cfg.Exchange(c.httpContext(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Token{}, tokenError(err, false)
	}
	c.Log.Printf("OAuth: exchanged authorization code (expires %s)", expiryText(tok.Expiry))
	return toToken(tok, scopes), nil
}

// Preflight exchanges a dummy code to catch the two common setup errors, an
// unsupported Redmine and wrong client credentials, before a browser opens.
// Any other OAuth error (expected: invalid_grant) means the endpoint and the
// client are fine.
func (c *Client) Preflight(ctx context.Context) error {
	cfg := c.oauth2Config(preflightRedirect, nil)
	_, err := cfg.Exchange(c.httpContext(ctx), "redmine-cli-preflight")
	if err == nil {
		return nil
	}
	var re *oauth2.RetrieveError
	if !errors.As(err, &re) {
		return err
	}
	if mapped := tokenError(err, false); errors.Is(mapped, ErrUnsupported) || errors.Is(mapped, ErrInvalidClient) {
		return mapped
	}
	return nil
}

// Refresh trades a refresh token for a new token pair. When Redmine returns no
// new refresh token the old one is kept.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	cfg := c.oauth2Config("", nil)
	tok, err := cfg.TokenSource(c.httpContext(ctx), &oauth2.Token{RefreshToken: refreshToken}).Token()
	if err != nil {
		return Token{}, tokenError(err, true)
	}
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken
	}
	return toToken(tok, nil), nil
}

// Revoke revokes a token and its pair at Redmine.
func (c *Client) Revoke(ctx context.Context, token string) error {
	form := url.Values{"token": {token}, "token_type_hint": {"refresh_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/oauth/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(c.ClientID), url.QueryEscape(c.ClientSecret))
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("revoking the OAuth token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revoking the OAuth token: HTTP %d", resp.StatusCode)
	}
	c.Log.Printf("OAuth: revoked token")
	return nil
}

func expiryText(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}
