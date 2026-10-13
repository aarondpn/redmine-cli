package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/cmd"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/update"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func TestWaitForStartupUpdate_FastResult_NoHint(t *testing.T) {
	updateDone := make(chan *update.CheckResult, 1)
	updateDone <- &update.CheckResult{
		NewVersion: "2.0.0",
		ReleaseURL: "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
	}

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, nil, 10*time.Millisecond, 50*time.Millisecond)

	out := buf.String()
	if strings.Contains(out, "Checking for updates...") {
		t.Fatalf("unexpected progress hint in output: %q", out)
	}
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v2.0.0") {
		t.Fatalf("expected update notice in output, got %q", out)
	}
}

func TestWaitForStartupUpdate_SlowResult_PrintsHintAndNotice(t *testing.T) {
	updateDone := make(chan *update.CheckResult, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		updateDone <- &update.CheckResult{
			NewVersion: "2.0.0",
			ReleaseURL: "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		}
	}()

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, nil, 10*time.Millisecond, 100*time.Millisecond)

	out := buf.String()
	if !strings.Contains(out, "Checking for updates...") {
		t.Fatalf("expected progress hint in output, got %q", out)
	}
	if !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v2.0.0") {
		t.Fatalf("expected update notice in output, got %q", out)
	}
}

func TestWaitForStartupUpdate_SlowNilResult_PrintsHintOnly(t *testing.T) {
	updateDone := make(chan *update.CheckResult, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		updateDone <- nil
	}()

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, nil, 10*time.Millisecond, 100*time.Millisecond)

	out := buf.String()
	if !strings.Contains(out, "Checking for updates...") {
		t.Fatalf("expected progress hint in output, got %q", out)
	}
	if strings.Contains(out, "A new version of redmine is available") {
		t.Fatalf("did not expect update notice in output, got %q", out)
	}
}

func TestWaitForStartupUpdate_TimeoutCancelsCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	updateDone := make(chan *update.CheckResult, 1)
	canceled := make(chan struct{})

	go func() {
		<-ctx.Done()
		close(canceled)
	}()

	var buf bytes.Buffer
	waitForStartupUpdate(&buf, "1.0.0", updateDone, cancel, 10*time.Millisecond, 40*time.Millisecond)

	select {
	case <-canceled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected update check context to be canceled")
	}

	out := buf.String()
	if !strings.Contains(out, "Checking for updates...") {
		t.Fatalf("expected progress hint in output, got %q", out)
	}
	if strings.Contains(out, "A new version of redmine is available") {
		t.Fatalf("did not expect update notice in output, got %q", out)
	}
}

func TestSelectedOutputFormat_FallsBackToConfigDefault(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "server: https://example.invalid\napi_key: test\nauth_method: apikey\noutput_format: json\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	root, factory := cmd.NewRootCmdWithFactory("test")
	factory.ConfigPath = cfgPath

	got := selectedOutputFormat(root, factory, []string{"versions", "list"})
	if got != output.FormatJSON {
		t.Fatalf("selectedOutputFormat() = %q, want %q", got, output.FormatJSON)
	}
}

func TestSelectedOutputFormat_HonorsConfigFlagBeforePersistentPreRun(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "custom.yaml")
	content := "server: https://example.invalid\napi_key: test\nauth_method: apikey\noutput_format: json\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	root, factory := cmd.NewRootCmdWithFactory("test")
	if err := root.PersistentFlags().Set("config", cfgPath); err != nil {
		t.Fatal(err)
	}

	got := selectedOutputFormat(root, factory, []string{"versions", "get"})
	if got != output.FormatJSON {
		t.Fatalf("selectedOutputFormat() = %q, want %q", got, output.FormatJSON)
	}
}

func TestSelectedOutputFormat_HonorsProfileFlagBeforePersistentPreRun(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "profiles.yaml")
	content := `profiles:
  a:
    server: https://a.invalid
    auth_method: apikey
    api_key: a
    output_format: table
  b:
    server: https://b.invalid
    auth_method: apikey
    api_key: b
    output_format: json
active_profile: a
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	root, factory := cmd.NewRootCmdWithFactory("test")
	if err := root.PersistentFlags().Set("config", cfgPath); err != nil {
		t.Fatal(err)
	}
	if err := root.PersistentFlags().Set("profile", "b"); err != nil {
		t.Fatal(err)
	}

	got := selectedOutputFormat(root, factory, []string{"versions", "get"})
	if got != output.FormatJSON {
		t.Fatalf("selectedOutputFormat() = %q, want %q", got, output.FormatJSON)
	}
}

func TestSelectedOutputFormat_UsesClearedFactoryFormatForInteractiveRejection(t *testing.T) {
	root, factory := cmd.NewRootCmdWithFactory("test")
	factory.OutputFormat = output.FormatTable

	if err := root.PersistentFlags().Set("output", output.FormatJSON); err != nil {
		t.Fatal(err)
	}

	got := selectedOutputFormat(root, factory, []string{"issues", "browse", "--output", "json"})
	if got != output.FormatTable {
		t.Fatalf("selectedOutputFormat() = %q, want %q", got, output.FormatTable)
	}
}

// TestMain lets tests re-execute the test binary as the real CLI: when
// REDMINE_MAIN_HELPER is set, os.Args[1:] are CLI arguments and main runs
// instead of the test suite.
func TestMain(m *testing.M) {
	if os.Getenv("REDMINE_MAIN_HELPER") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runCLI executes the CLI in a subprocess with a throwaway config and
// returns stdout, stderr, and the exit code. REDMINE_* variables from the
// developer's environment are dropped so the CLI can never reach a real
// server.
func runCLI(t *testing.T, serverURL string, args ...string) (string, string, int) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "server: " + serverURL + "\napi_key: test\nauth_method: apikey\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], append([]string{"--config", cfgPath}, args...)...)
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "REDMINE_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "REDMINE_MAIN_HELPER=1", "REDMINE_NO_UPDATE_CHECK=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run CLI: %v", err)
	}
	return stdout.String(), stderr.String(), code
}

func TestErrorsGoToStderrWithEmptyStdout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["Subject cannot be blank"]}`))
	}))
	defer srv.Close()

	tests := []struct {
		name     string
		args     []string
		wantJSON bool
		wantCode string
		wantMsg  string
	}{
		{
			name:     "failed write in JSON mode",
			args:     []string{"api", "/issues.json", "-F", "issue[subject]=", "-o", "json"},
			wantJSON: true,
			wantCode: output.ErrCodeValidationFailed,
			wantMsg:  "Subject cannot be blank",
		},
		{
			name:     "envelope is the last stderr line after --verbose output",
			args:     []string{"api", "/issues.json", "-F", "issue[subject]=", "-o", "json", "--verbose"},
			wantJSON: true,
			wantCode: output.ErrCodeValidationFailed,
			wantMsg:  "Subject cannot be blank",
		},
		{
			name:     "failed write is not filtered by --jq",
			args:     []string{"api", "/issues.json", "-F", "issue[subject]=", "--jq", "{id}"},
			wantJSON: true,
			wantMsg:  "Subject cannot be blank",
		},
		{
			name:     "argument validation in JSON mode",
			args:     []string{"issues", "get", "-o", "json"},
			wantJSON: true,
			wantMsg:  "accepts 1 arg(s)",
		},
		{
			name:     "argument validation with --jq",
			args:     []string{"issues", "get", "--jq", ".id"},
			wantJSON: true,
			wantMsg:  "accepts 1 arg(s)",
		},
		{
			name:     "invalid --jq expression",
			args:     []string{"config", "--jq", ".["},
			wantJSON: true,
			wantCode: output.ErrCodeJQInvalid,
			wantMsg:  "invalid --jq expression",
		},
		{
			name:     "explicitly empty --jq expression",
			args:     []string{"config", "--jq", ""},
			wantJSON: true,
			wantCode: output.ErrCodeJQInvalid,
			wantMsg:  "invalid --jq expression: expression is empty",
		},
		{
			// The command ends with a SilentError (non-2xx with -i), which
			// skips PersistentPostRunE; the jq failure must still surface.
			name:     "--jq runtime error before a silent failure",
			args:     []string{"api", "/issues.json", "-i", "--jq", ".[0]"},
			wantJSON: true,
			wantCode: output.ErrCodeJQFailed,
			wantMsg:  "command succeeded, but --jq failed: ",
		},
		{
			name:     "--jq halt_error with a non-zero exit code",
			args:     []string{"config", "--jq", "null | halt_error(5)"},
			wantJSON: true,
			wantCode: output.ErrCodeJQFailed,
			wantMsg:  "command succeeded, but --jq failed: ",
		},
		{
			name:     "--jq runtime error",
			args:     []string{"config", "--jq", ".[0]"},
			wantJSON: true,
			wantCode: output.ErrCodeJQFailed,
			wantMsg:  "command succeeded, but --jq failed: ",
		},
		{
			name:    "table mode",
			args:    []string{"issues", "get", "-o", "table"},
			wantMsg: "Error: accepts 1 arg(s)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, srv.URL, tt.args...)
			if code == 0 {
				t.Fatalf("expected non-zero exit, stderr:\n%s", stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout on failure, got:\n%s", stdout)
			}
			if !strings.Contains(stderr, tt.wantMsg) {
				t.Fatalf("stderr missing %q:\n%s", tt.wantMsg, stderr)
			}
			if !tt.wantJSON {
				return
			}
			// The envelope is one compact line and always the last line of
			// stderr, whatever was logged before it.
			lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
			if slices.Contains(tt.args, "--verbose") && len(lines) < 2 {
				t.Fatalf("expected debug output before the envelope:\n%s", stderr)
			}
			var env output.ErrorEnvelope
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &env); err != nil {
				t.Fatalf("last stderr line is not an error envelope: %v\n%s", err, stderr)
			}
			if env.Error.Message == "" {
				t.Fatalf("error envelope missing message:\n%s", stderr)
			}
			if tt.wantCode != "" && env.Error.Code != tt.wantCode {
				t.Fatalf("error code = %q, want %q", env.Error.Code, tt.wantCode)
			}
		})
	}
}
