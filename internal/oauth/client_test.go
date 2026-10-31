package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/oauth/oauthtest"
)

func newClient(srv *oauthtest.Server) *Client {
	return &Client{
		Server:       srv.URL,
		ClientID:     srv.ClientID,
		ClientSecret: srv.ClientSecret,
		HTTPClient:   srv.Client(),
	}
}

func TestPreflight(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		secret  string
		wantErr error
	}{
		{name: "invalid_grant is fine"},
		{name: "wrong secret", secret: "nope", wantErr: ErrInvalidClient},
		{
			name: "no oauth routes",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			},
			wantErr: ErrUnsupported,
		},
		{
			name: "other oauth error is fine",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := oauthtest.NewServer(t)
			c := newClient(srv)
			if tt.handler != nil {
				other := httptest.NewServer(tt.handler)
				defer other.Close()
				c.Server = other.URL
				c.HTTPClient = other.Client()
			}
			if tt.secret != "" {
				c.ClientSecret = tt.secret
			}
			err := c.Preflight(context.Background())
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Preflight() = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Preflight() = %v, want %v", err, tt.wantErr)
			}
		})
	}

	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		c := &Client{Server: srv.URL, ClientID: "a", ClientSecret: "b"}
		srv.Close()
		err := c.Preflight(context.Background())
		if err == nil || errors.Is(err, ErrUnsupported) || errors.Is(err, ErrInvalidClient) {
			t.Fatalf("Preflight() = %v, want a transport error", err)
		}
	})
}

func TestExchange(t *testing.T) {
	srv := oauthtest.NewServer(t)
	srv.ExpiresIn = 600
	c := newClient(srv)
	scopes := []string{"view_issues", "log_time"}
	f, err := c.Start(scopes)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	srv.Approve(t, f.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tok, err := f.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("token = %+v", tok)
	}
	if !slices.Equal(tok.Scopes, scopes) {
		t.Errorf("Scopes = %v, want %v", tok.Scopes, scopes)
	}
	if d := time.Until(tok.Expiry); d < 9*time.Minute || d > 10*time.Minute {
		t.Errorf("Expiry in %v, want about 10m", d)
	}
}

func TestExchangeWrongVerifier(t *testing.T) {
	srv := oauthtest.NewServer(t)
	c := newClient(srv)
	f, err := c.Start([]string{"view_issues"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	srv.Approve(t, f.URL())
	f.verifier = "x" + f.verifier[1:]
	_, err = f.Wait(context.Background())
	if err == nil || !strings.Contains(err.Error(), "authorization code") {
		t.Fatalf("Wait() = %v, want a rejected authorization code", err)
	}
}

func TestRefreshRotates(t *testing.T) {
	srv := oauthtest.NewServer(t)
	c := newClient(srv)
	access, refresh := srv.Issue([]string{"view_issues"})

	next, err := c.Refresh(context.Background(), refresh)
	if err != nil {
		t.Fatal(err)
	}
	if next.AccessToken == access || next.RefreshToken == refresh || next.RefreshToken == "" {
		t.Errorf("tokens did not rotate: %+v", next)
	}
	if !slices.Equal(next.Scopes, []string{"view_issues"}) {
		t.Errorf("Scopes = %v", next.Scopes)
	}
	if srv.Refreshes.Load() != 1 {
		t.Errorf("Refreshes = %d, want 1", srv.Refreshes.Load())
	}
}

func TestRefreshReusedRefreshTokenNeedsLogin(t *testing.T) {
	srv := oauthtest.NewServer(t)
	c := newClient(srv)
	_, refresh := srv.Issue([]string{"view_issues"})
	next, err := c.Refresh(context.Background(), refresh)
	if err != nil {
		t.Fatal(err)
	}
	// Using the new access token revokes the old refresh token.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/users/current.json", nil)
	req.Header.Set("Authorization", "Bearer "+next.AccessToken)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	_, err = c.Refresh(context.Background(), refresh)
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Refresh(reused) = %v, want ErrLoginRequired", err)
	}
}

func TestRefreshKeepsRefreshTokenWhenNoneReturned(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new","token_type":"Bearer","expires_in":60}`))
	}))
	defer ts.Close()
	c := &Client{Server: ts.URL, ClientID: "a", ClientSecret: "b"}
	tok, err := c.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "new" || tok.RefreshToken != "old-refresh" {
		t.Errorf("token = %+v", tok)
	}
}

func TestRevoke(t *testing.T) {
	srv := oauthtest.NewServer(t)
	c := newClient(srv)
	access, refresh := srv.Issue([]string{"view_issues"})

	if err := c.Revoke(context.Background(), refresh); err != nil {
		t.Fatal(err)
	}
	if srv.Revokes.Load() != 1 {
		t.Errorf("Revokes = %d, want 1", srv.Revokes.Load())
	}
	if _, err := c.Refresh(context.Background(), refresh); !errors.Is(err, ErrLoginRequired) {
		t.Errorf("Refresh after revoke = %v, want ErrLoginRequired", err)
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/users/current.json", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("revoked access token status = %d, want 401", resp.StatusCode)
	}

	bad := newClient(srv)
	bad.ClientSecret = "nope"
	if err := bad.Revoke(context.Background(), refresh); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("Revoke with bad secret = %v, want HTTP 401", err)
	}
}

func TestTokenError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		refresh bool
		want    error
		text    string
	}{
		{"404", 404, "", false, ErrUnsupported, ""},
		{"invalid_client", 401, `{"error":"invalid_client"}`, false, ErrInvalidClient, ""},
		{"401 without body", 401, "", true, ErrInvalidClient, ""},
		{"invalid_scope", 400, `{"error":"invalid_scope"}`, false, ErrInvalidScope, ""},
		{"invalid_grant refresh", 400, `{"error":"invalid_grant"}`, true, ErrLoginRequired, ""},
		{"invalid_grant code", 400, `{"error":"invalid_grant","error_description":"bad code"}`, false, nil, "Redmine rejected the authorization code: bad code"},
		{"other", 400, `{"error":"server_error","error_description":"x\u001b[2Jy"}`, false, nil, "OAuth token request failed: server_error: x^[[2Jy"},
		{"no body", 500, "", false, nil, "OAuth token request failed: HTTP 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()
			c := &Client{Server: ts.URL, ClientID: "a", ClientSecret: "b"}
			_, err := c.Refresh(context.Background(), "r")
			if !tt.refresh {
				_, err = c.exchange(context.Background(), "code", "verifier", "http://127.0.0.1/callback", nil)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if tt.text != "" && (err == nil || err.Error() != tt.text) {
				t.Fatalf("err = %v, want %q", err, tt.text)
			}
		})
	}
}
