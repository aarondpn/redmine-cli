package mcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

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
		want := "redmine auth login"
		if status == http.StatusForbidden {
			want = "Enable REST web service"
		}
		if !strings.Contains(out, cfg.Server) || !strings.Contains(out, want) {
			t.Errorf("status %d: warning = %q, want server and %q", status, out, want)
		}
		if status == http.StatusForbidden && strings.Contains(out, "auth login") {
			t.Errorf("403 warning must not suggest re-login: %q", out)
		}
	}
}

func TestCredentialCheck_ScrubsServerURL(t *testing.T) {
	client, cfg := newProbeClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	cfg.Server = strings.Replace(cfg.Server, "http://", "http://proxyuser:proxypass@", 1) + "/?key=abc"
	f, stderr := newProbeFactory(t)
	<-startCredentialCheck(context.Background(), f, cfg, client)
	out := stderr.String()
	if out == "" || strings.Contains(out, "proxypass") || strings.Contains(out, "key=abc") {
		t.Errorf("warning = %q, must be non-empty and free of credentials", out)
	}
}

func TestCredentialWarning_MissingSecret(t *testing.T) {
	err := &api.APIError{StatusCode: http.StatusUnauthorized}
	got := credentialWarning(&config.Config{Server: "https://r.example.com", AuthMethod: "apikey"}, err)
	if !strings.Contains(got, "no API key is configured") || !strings.Contains(got, "REDMINE_API_KEY") {
		t.Errorf("apikey: %q", got)
	}
	got = credentialWarning(&config.Config{Server: "https://r.example.com", AuthMethod: "basic", Password: "x"}, err)
	if !strings.Contains(got, "password is wrong") || !strings.Contains(got, "REDMINE_PASSWORD") {
		t.Errorf("basic: %q", got)
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

func TestHTTPRequested(t *testing.T) {
	newCmd := func(args ...string) *cobra.Command {
		c := newCmdServe(cmdutil.NewFactory())
		if err := c.ParseFlags(args); err != nil {
			t.Fatal(err)
		}
		return c
	}
	t.Setenv("REDMINE_MCP_HTTP", "")
	if httpRequested(newCmd(), "", nil, errUnknownProfile{}) {
		t.Error("no flag, env or config means stdio")
	}
	if !httpRequested(newCmd("--http", ":0"), ":0", nil, errUnknownProfile{}) {
		t.Error("--http must count")
	}
	if httpRequested(newCmd("--http", ""), "", nil, nil) {
		t.Error("explicit empty --http means stdio")
	}
	if !httpRequested(newCmd(), "", &config.Config{MCP: config.MCPConfig{HTTP: ":1"}}, nil) {
		t.Error("loaded config mcp.http must count")
	}
	t.Setenv("REDMINE_MCP_HTTP", "127.0.0.1:0")
	if !httpRequested(newCmd(), "", nil, errUnknownProfile{}) {
		t.Error("REDMINE_MCP_HTTP must count when the config failed to load")
	}
}

// keyringProfileConfig has a keyring profile whose secret is unavailable
// (REDMINE_NO_KEYRING is set).
const keyringProfileConfig = "active_profile: work\nprofiles:\n  work:\n    server: https://work.example.com\n    auth_method: apikey\n    credential_store: keyring\n    keyring_id: srv-id\n"

// runServeUnconfigured runs `mcp serve` against keyringProfileConfig, with
// stdin at EOF.
func runServeUnconfigured(t *testing.T, profile string, env map[string]string, args ...string) (stderr string, err error) {
	t.Helper()
	return runServeWithConfig(t, keyringProfileConfig, profile, env, args...)
}

// runServeWithConfig runs `mcp serve` against a config file with content,
// with stdin at EOF.
func runServeWithConfig(t *testing.T, content, profile string, env map[string]string, args ...string) (stderr string, err error) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "REDMINE_") {
			t.Setenv(k, "")
		}
	}
	t.Setenv("REDMINE_NO_KEYRING", "1")
	for k, v := range env {
		t.Setenv(k, v)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if werr := os.WriteFile(cfgPath, []byte(content), 0o600); werr != nil {
		t.Fatal(werr)
	}
	var errBuf bytes.Buffer
	f := &cmdutil.Factory{
		ConfigPath:      cfgPath,
		ProfileOverride: profile,
		IOStreams:       &cmdutil.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &errBuf},
	}
	cmd := newCmdServe(f)
	cmd.SetArgs(args)
	cmd.SetErr(&errBuf)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	err = cmd.ExecuteContext(context.Background())
	return errBuf.String(), err
}

// closedStdio replaces the stdio transport with one whose client side is
// already gone, so the setup server returns right after starting.
func closedStdio(t *testing.T) {
	t.Helper()
	orig := newStdioTransport
	t.Cleanup(func() { newStdioTransport = orig })
	newStdioTransport = func() sdk.Transport {
		r, w := io.Pipe()
		_ = w.Close() // client side gone: immediate EOF
		return &sdk.IOTransport{Reader: r, Writer: nopWriteCloser{io.Discard}}
	}
}

func TestServeUnconfigured(t *testing.T) {
	t.Run("stdio starts the setup server", func(t *testing.T) {
		closedStdio(t)
		out, err := runServeUnconfigured(t, "", nil)
		if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "EOF") {
			t.Fatalf("err = %v, want a clean stdio shutdown", err)
		}
		if !strings.Contains(out, "not configured") || !strings.Contains(out, "redmine auth login") {
			t.Errorf("stderr = %q, want the not-configured notice pointing at auth login", out)
		}
	})
	t.Run("no active profile points at auth switch", func(t *testing.T) {
		closedStdio(t)
		content := "profiles:\n  a:\n    server: https://a.example.com\n    api_key: k\n  b:\n    server: https://b.example.com\n    api_key: k\n"
		out, err := runServeWithConfig(t, content, "", nil)
		if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "EOF") {
			t.Fatalf("err = %v, want a clean stdio shutdown", err)
		}
		if !strings.Contains(out, "not configured") || !strings.Contains(out, "redmine auth switch") || strings.Contains(out, "redmine auth login") {
			t.Errorf("stderr = %q, want the not-configured notice pointing at auth switch", out)
		}
	})
	t.Run("--http fails with the error", func(t *testing.T) {
		_, err := runServeUnconfigured(t, "", nil, "--http", "127.0.0.1:0")
		if err == nil || !strings.Contains(err.Error(), "keyring") {
			t.Fatalf("err = %v, want the keyring error", err)
		}
	})
	t.Run("REDMINE_MCP_HTTP fails with the error", func(t *testing.T) {
		_, err := runServeUnconfigured(t, "", map[string]string{"REDMINE_MCP_HTTP": "127.0.0.1:0"})
		if err == nil || !strings.Contains(err.Error(), "keyring") {
			t.Fatalf("err = %v, want the keyring error", err)
		}
	})
	t.Run("unknown profile fails", func(t *testing.T) {
		_, err := runServeUnconfigured(t, "nope", nil)
		if err == nil {
			t.Fatal("want an error for an unknown profile")
		}
	})
	t.Run("invalid filter flag fails", func(t *testing.T) {
		_, err := runServeUnconfigured(t, "", nil, "--enable-groups", "bogus")
		if err == nil {
			t.Fatal("want an error for an invalid group")
		}
	})
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
