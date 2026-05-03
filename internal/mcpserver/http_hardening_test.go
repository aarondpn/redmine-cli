package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWrapAuthToken_AllowsValidBearer(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler := WrapAuthToken(inner, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler never invoked for valid token")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWrapAuthToken_AllowsCaseInsensitiveSchemeAndExtraSpaces(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler := WrapAuthToken(inner, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "bearer    s3cret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler never invoked for valid token")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWrapAuthToken_RejectsMissingHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("inner handler should not run when token is missing")
	})

	handler := WrapAuthToken(inner, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("expected WWW-Authenticate header on 401")
	}
}

func TestWrapAuthToken_RejectsWrongToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("inner handler should not run when token is wrong")
	})

	handler := WrapAuthToken(inner, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestWrapAuthToken_EmptyTokenIsPassThrough(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := WrapAuthToken(inner, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no gate)", rec.Code)
	}
}

func TestBuildHTTPServer_AppliesDefaultTimeouts(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer closeTS()

	srv := BuildHTTPServer(apiClient, Options{Version: "v0"}, HTTPOptions{Addr: "127.0.0.1:0"})

	if srv.Addr != "127.0.0.1:0" {
		t.Errorf("Addr = %q, want 127.0.0.1:0", srv.Addr)
	}
	if srv.ReadHeaderTimeout != defaultReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %s, want %s", srv.ReadHeaderTimeout, defaultReadHeaderTimeout)
	}
	if srv.ReadTimeout != defaultReadTimeout {
		t.Errorf("ReadTimeout = %s, want %s", srv.ReadTimeout, defaultReadTimeout)
	}
	if srv.IdleTimeout != defaultIdleTimeout {
		t.Errorf("IdleTimeout = %s, want %s", srv.IdleTimeout, defaultIdleTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %s, want 0 (intentionally unbounded for slow tool calls)", srv.WriteTimeout)
	}
	if srv.Handler == nil {
		t.Fatal("Handler is nil")
	}
}

func TestBuildHTTPServer_OverridesTakePrecedence(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer closeTS()

	srv := BuildHTTPServer(apiClient, Options{Version: "v0"}, HTTPOptions{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       4 * time.Second,
	})

	if srv.ReadHeaderTimeout != 1*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want 1s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 2*time.Second {
		t.Errorf("ReadTimeout = %s, want 2s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 3*time.Second {
		t.Errorf("WriteTimeout = %s, want 3s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 4*time.Second {
		t.Errorf("IdleTimeout = %s, want 4s", srv.IdleTimeout)
	}
}

func TestBuildHTTPServer_WrapsHandlerWhenAuthTokenSet(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer closeTS()

	srv := BuildHTTPServer(apiClient, Options{Version: "v0"}, HTTPOptions{
		Addr:      "127.0.0.1:0",
		AuthToken: "s3cret",
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer token, got %d", rec.Code)
	}
}

func TestNormalizeBindAddr(t *testing.T) {
	cases := []struct {
		in           string
		wantAddr     string
		wantLoopback bool
	}{
		{"", "", false},
		{":8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", "127.0.0.1:8080", true},
		{"localhost:8080", "localhost:8080", true},
		{"[::1]:8080", "[::1]:8080", true},
		{"0.0.0.0:8080", "0.0.0.0:8080", false},
		{"[::]:8080", "[::]:8080", false},
		{"10.0.0.5:8080", "10.0.0.5:8080", false},
		{"not-an-addr", "not-an-addr", false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			gotAddr, gotLoopback := NormalizeBindAddr(tc.in)
			if gotAddr != tc.wantAddr {
				t.Errorf("addr = %q, want %q", gotAddr, tc.wantAddr)
			}
			if gotLoopback != tc.wantLoopback {
				t.Errorf("isLoopback = %v, want %v", gotLoopback, tc.wantLoopback)
			}
		})
	}
}
