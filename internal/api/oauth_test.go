package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth/oauthtest"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
)

func oauthTestConfig(srv *oauthtest.Server, access, refresh string, expiry time.Time) *config.Config {
	return &config.Config{
		Server:     srv.URL,
		AuthMethod: config.AuthMethodOAuth,
		OAuth: config.OAuthConfig{
			ClientID:     srv.ClientID,
			ClientSecret: srv.ClientSecret,
			AccessToken:  access,
			RefreshToken: refresh,
			Expiry:       expiry,
			Scopes:       []string{"view_issues"},
		},
	}
}

// issuesEndpoint serves /issues.json behind RequireBearer and records the
// request bodies it accepted.
func issuesEndpoint(srv *oauthtest.Server) (bodies *[]string) {
	bodies = new([]string)
	srv.Mux.Handle("/issues.json", srv.RequireBearer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(data))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[]}`))
	})))
	return bodies
}

func TestOAuthTransport_SendsBearerNotAPIKey(t *testing.T) {
	srv := oauthtest.NewServer(t)
	access, refresh := srv.Issue([]string{"view_issues"})
	var gotAuth, gotKey string
	srv.Mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotKey = r.Header.Get("Authorization"), r.Header.Get("X-Redmine-API-Key")
		_, _ = w.Write([]byte("{}"))
	})
	client, err := NewClient(oauthTestConfig(srv, access, refresh, time.Now().Add(time.Hour)), debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(), "/probe", nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer "+access {
		t.Errorf("Authorization = %q, want Bearer %s", gotAuth, access)
	}
	if gotKey != "" {
		t.Errorf("X-Redmine-API-Key = %q, want empty", gotKey)
	}
}

func TestAuthTransport_OAuthBearerStaysOnConfiguredOrigin(t *testing.T) {
	srv := oauthtest.NewServer(t)
	access, refresh := srv.Issue(nil)
	cfg := oauthTestConfig(srv, access, refresh, time.Now().Add(time.Hour))
	tokens := newTokenSource(cfg, debug.New(nil), newBaseTransport(0))

	var gotAuth string
	srv.Mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("{}"))
	})
	// Configured host is deliberately different from the server we call.
	foreign := &http.Client{Transport: &authTransport{
		base: http.DefaultTransport, authMethod: config.AuthMethodOAuth, tokens: tokens,
		origin: &config.Origin{Scheme: "http", Host: "redmine.internal", Port: "9999"},
	}}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/probe", nil)
	resp, err := foreign.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty (must not leak to a foreign host)", gotAuth)
	}
}

func TestOAuthTransport_RefreshesNearExpiryOnce(t *testing.T) {
	srv := oauthtest.NewServer(t)
	access, refresh := srv.Issue([]string{"view_issues"})
	issuesEndpoint(srv)
	client, err := NewClient(oauthTestConfig(srv, access, refresh, time.Now().Add(10*time.Second)), debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := client.Get(context.Background(), "/issues.json", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if n := srv.Refreshes.Load(); n != 1 {
		t.Errorf("refreshes = %d, want 1", n)
	}
	tok, ok := client.OAuthToken()
	if !ok || tok.AccessToken == access {
		t.Errorf("OAuthToken() = %+v, %v; want the refreshed token", tok, ok)
	}
}

func TestOAuthTransport_RetriesAfter401WithBody(t *testing.T) {
	srv := oauthtest.NewServer(t)
	access, refresh := srv.Issue([]string{"add_issues"})
	bodies := issuesEndpoint(srv)
	client, err := NewClient(oauthTestConfig(srv, access, refresh, time.Now().Add(time.Hour)), debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	srv.ExpireAccessTokens()
	if err := client.Post(context.Background(), "/issues.json", map[string]string{"subject": "x"}, nil); err != nil {
		t.Fatal(err)
	}
	if n := srv.Refreshes.Load(); n != 1 {
		t.Errorf("refreshes = %d, want 1", n)
	}
	if len(*bodies) != 1 || strings.TrimSpace((*bodies)[0]) != `{"subject":"x"}` {
		t.Errorf("accepted bodies = %q, want the JSON body replayed once", *bodies)
	}
}

func TestOAuthTransport_NoRetryForNonReplayableBody(t *testing.T) {
	srv := oauthtest.NewServer(t)
	access, refresh := srv.Issue([]string{"add_issues"})
	issuesEndpoint(srv)
	client, err := NewClient(oauthTestConfig(srv, access, refresh, time.Now().Add(time.Hour)), debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	srv.ExpireAccessTokens()
	body := io.NopCloser(bytes.NewReader([]byte("data")))
	resp, err := client.DoRaw(context.Background(), http.MethodPost, "/issues.json", nil, body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if n := srv.Refreshes.Load(); n != 0 {
		t.Errorf("refreshes = %d, want 0", n)
	}
}

func TestOAuthTransport_RevokedRefreshTokenNeedsLogin(t *testing.T) {
	srv := oauthtest.NewServer(t)
	access, _ := srv.Issue(nil)
	issuesEndpoint(srv)
	client, err := NewClient(oauthTestConfig(srv, access, "revoked", time.Now().Add(-time.Minute)), debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	err = client.Get(context.Background(), "/issues.json", nil, nil)
	if !errors.Is(err, oauth.ErrLoginRequired) {
		t.Fatalf("err = %v, want ErrLoginRequired", err)
	}
	if strings.Contains(err.Error(), "request failed") {
		t.Errorf("err = %q, want no request failed wrapping", err)
	}
}

func TestNewClient_OAuthWithoutTokens(t *testing.T) {
	cfg := &config.Config{Server: "https://r.example.com", AuthMethod: config.AuthMethodOAuth, OAuth: config.OAuthConfig{ClientID: "c"}}
	_, err := NewClient(cfg, debug.New(nil))
	if err == nil || !strings.Contains(err.Error(), "redmine auth login --oauth") {
		t.Errorf("err = %v, want the login hint", err)
	}
}

func TestClient_OAuthTokenOtherMethods(t *testing.T) {
	client, err := NewClient(&config.Config{Server: "https://r.example.com", AuthMethod: "apikey", APIKey: "k"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := client.OAuthToken(); ok {
		t.Error("OAuthToken ok = true for an API key profile")
	}
}

func TestNewOAuthClient_SendsNoRedmineCredentials(t *testing.T) {
	var hdr http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Clone()
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()
	srv := &oauthtest.Server{Server: ts, ClientID: "c", ClientSecret: "s"}
	refresh := "r"
	cfg := oauthTestConfig(srv, "", refresh, time.Time{})
	cfg.APIKey = "api-key"
	cfg.Headers = map[string]string{"X-Proxy-Token": "p"}
	oc, err := NewOAuthClient(cfg, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = oc.Refresh(context.Background(), refresh)
	if hdr == nil {
		t.Fatal("token endpoint not called")
	}
	if hdr.Get("X-Proxy-Token") != "p" || hdr.Get("User-Agent") != UserAgent {
		t.Errorf("headers = %v, want X-Proxy-Token and User-Agent", hdr)
	}
	if got := hdr.Get("Authorization"); strings.HasPrefix(got, "Bearer") {
		t.Errorf("Authorization = %q, want no bearer token", got)
	}
	if hdr.Get("X-Redmine-API-Key") != "" {
		t.Errorf("X-Redmine-API-Key = %q, want empty", hdr.Get("X-Redmine-API-Key"))
	}
}

func TestOAuthProfile_PersistsRotatedTokens(t *testing.T) {
	for _, store := range []string{"", config.CredentialStoreKeyring} {
		t.Run("store="+store, func(t *testing.T) {
			keyring.MockInit()
			srv := oauthtest.NewServer(t)
			access, refresh := srv.Issue([]string{"view_issues"})
			issuesEndpoint(srv)
			cfg := oauthTestConfig(srv, access, refresh, time.Now().Add(-time.Minute))
			cfg.CredentialStore = store
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := config.SaveProfile("work", cfg, path); err != nil {
				t.Fatal(err)
			}
			loaded, err := config.Load(path, "work", debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			client, err := NewClient(loaded, debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Get(context.Background(), "/issues.json", nil, nil); err != nil {
				t.Fatal(err)
			}
			now, _ := client.OAuthToken()
			if now.AccessToken == access {
				t.Fatal("token was not refreshed")
			}

			stored, err := config.LoadOAuthTokens(path, "work")
			if err != nil {
				t.Fatal(err)
			}
			if stored.AccessToken != now.AccessToken || stored.RefreshToken != now.RefreshToken || stored.RefreshToken == refresh {
				t.Errorf("stored = %+v, want the rotated pair %+v", stored, now)
			}
			if store == config.CredentialStoreKeyring {
				pc, err := config.LoadProfiles(path, debug.New(nil))
				if err != nil {
					t.Fatal(err)
				}
				got, ok, err := secrets.Default.Get(pc.Profiles["work"].KeyringID, secrets.FieldOAuthRefreshToken)
				if err != nil || !ok || got != now.RefreshToken {
					t.Errorf("keyring refresh token = %q, %v, %v; want %q", got, ok, err, now.RefreshToken)
				}
			}
		})
	}
}
