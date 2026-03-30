package update

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestShouldCheck(t *testing.T) {
	// Override isTerminal to always return true for these tests.
	origTerminal := isTerminal
	t.Cleanup(func() { isTerminal = origTerminal })
	isTerminal = func() bool { return true }

	tests := []struct {
		name    string
		version string
		args    []string
		env     string // REDMINE_NO_UPDATE_CHECK value
		tty     bool
		want    bool
	}{
		{name: "normal", version: "1.0.0", args: []string{"issue", "list"}, tty: true, want: true},
		{name: "dev version", version: "dev", args: nil, tty: true, want: false},
		{name: "env 1", version: "1.0.0", args: nil, env: "1", tty: true, want: false},
		{name: "env true", version: "1.0.0", args: nil, env: "true", tty: true, want: false},
		{name: "env TRUE", version: "1.0.0", args: nil, env: "TRUE", tty: true, want: false},
		{name: "update command", version: "1.0.0", args: []string{"update"}, tty: true, want: false},
		{name: "update after flags", version: "1.0.0", args: []string{"--verbose", "update"}, tty: true, want: false},
		{name: "update after flag with value", version: "1.0.0", args: []string{"--config", "cfg.yaml", "update"}, tty: true, want: false},
		{name: "update after short flag", version: "1.0.0", args: []string{"-v", "update"}, tty: true, want: false},
		{name: "no tty", version: "1.0.0", args: nil, tty: false, want: false},
		{name: "no args", version: "1.0.0", args: nil, tty: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTerminal = func() bool { return tt.tty }

			if tt.env != "" {
				t.Setenv("REDMINE_NO_UPDATE_CHECK", tt.env)
			}
			got := ShouldCheck(tt.version, tt.args)
			if got != tt.want {
				t.Errorf("ShouldCheck(%q, %v) = %v, want %v", tt.version, tt.args, got, tt.want)
			}
		})
	}
}

func stubFetchRelease(t *testing.T, fn func(ctx context.Context) (*GithubRelease, error)) {
	t.Helper()
	orig := fetchRelease
	t.Cleanup(func() { fetchRelease = orig })
	fetchRelease = fn
}

func TestCheckForUpdate_NewerAvailable(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})

	result := CheckForUpdate(context.Background(), "1.0.0")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q, want %q", result.NewVersion, "2.0.0")
	}
	if !strings.Contains(result.ReleaseURL, "v2.0.0") {
		t.Errorf("ReleaseURL = %q, want it to contain v2.0.0", result.ReleaseURL)
	}
}

func TestCheckForUpdate_AlreadyUpToDate(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v1.0.0"}, nil
	})

	result := CheckForUpdate(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result for same version, got %+v", result)
	}
}

func TestCheckForUpdate_NetworkError(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return nil, fmt.Errorf("network error")
	})

	result := CheckForUpdate(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
}

func TestCheckForUpdate_Timeout(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return &GithubRelease{TagName: "v2.0.0"}, nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result := CheckForUpdate(ctx, "1.0.0")
	if result != nil {
		t.Errorf("expected nil result on timeout, got %+v", result)
	}
}

func TestPrintNotice(t *testing.T) {
	var buf bytes.Buffer
	result := &CheckResult{
		NewVersion: "2.0.0",
		ReleaseURL: "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
	}
	PrintNotice(&buf, "1.0.0", result)

	out := buf.String()
	for _, want := range []string{"v1.0.0", "v2.0.0", "redmine update", result.ReleaseURL} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintNotice_Nil(t *testing.T) {
	var buf bytes.Buffer
	PrintNotice(&buf, "1.0.0", nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil result, got %q", buf.String())
	}
}
