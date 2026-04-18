package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
