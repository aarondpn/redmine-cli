package mcp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func newProbeFactory(t *testing.T) (*cmdutil.Factory, *bytes.Buffer) {
	t.Helper()
	var stderr bytes.Buffer
	f := cmdutil.NewFactory()
	f.IOStreams = &cmdutil.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &stderr}
	return f, &stderr
}

func newProbeClient(t *testing.T, h http.HandlerFunc) (*api.Client, *config.Config) {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	cfg := &config.Config{Server: ts.URL, APIKey: "k", AuthMethod: "apikey"}
	client, err := api.NewClient(cfg, debug.New(nil))
	if err != nil {
		t.Fatalf("api.NewClient: %v", err)
	}
	return client, cfg
}

func TestCredentialCheck_WarnsOnRejectedCredentials(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		client, cfg := newProbeClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		})
		f, stderr := newProbeFactory(t)
		<-startCredentialCheck(context.Background(), f, cfg, client)
		out := stderr.String()
		if !strings.Contains(out, cfg.Server) || !strings.Contains(out, "redmine auth login") {
			t.Errorf("status %d: warning = %q, want server and fix", status, out)
		}
	}
}

func TestCredentialCheck_SilentWhenHealthyOrUnclear(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError, http.StatusNotFound} {
		client, cfg := newProbeClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if status == http.StatusOK {
				_, _ = w.Write([]byte(`{"user":{"id":1,"login":"a"}}`))
			}
		})
		f, stderr := newProbeFactory(t)
		<-startCredentialCheck(context.Background(), f, cfg, client)
		if stderr.Len() != 0 {
			t.Errorf("status %d: unexpected warning %q", status, stderr.String())
		}
	}
}

func TestCredentialCheck_DoesNotBlock(t *testing.T) {
	release := make(chan struct{})
	client, cfg := newProbeClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusUnauthorized)
	})
	f, _ := newProbeFactory(t)

	returned := make(chan (<-chan struct{}), 1)
	go func() { returned <- startCredentialCheck(context.Background(), f, cfg, client) }()
	select {
	case done := <-returned:
		close(release)
		<-done
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("startCredentialCheck blocked on a slow server")
	}
}

func TestIsSetupProblem(t *testing.T) {
	if !isSetupProblem(&config.Config{}, nil) {
		t.Error("empty server must count as a setup problem")
	}
	if isSetupProblem(&config.Config{Server: "https://r.example.com"}, nil) {
		t.Error("configured server is not a setup problem")
	}
	if isSetupProblem(nil, errUnknownProfile{}) {
		t.Error("an unknown profile is the user's explicit error")
	}
}

type errUnknownProfile struct{}

func (errUnknownProfile) Error() string { return `profile "x" not found` }
