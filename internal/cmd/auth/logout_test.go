package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
)

func TestResolveLogoutProfileName_HonorsProfileOverride(t *testing.T) {
	pc := &config.ProfileConfig{
		ActiveProfile: "work",
		Profiles: map[string]config.Config{
			"work":     {Server: "https://work.example.com"},
			"personal": {Server: "https://personal.example.com"},
		},
	}

	got, err := resolveLogoutProfileName(pc, nil, "personal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "personal" {
		t.Fatalf("resolveLogoutProfileName() = %q, want %q", got, "personal")
	}
}

func TestResolveLogoutProfileName_ArgsTakePrecedence(t *testing.T) {
	pc := &config.ProfileConfig{
		ActiveProfile: "work",
		Profiles: map[string]config.Config{
			"work":     {Server: "https://work.example.com"},
			"personal": {Server: "https://personal.example.com"},
		},
	}

	got, err := resolveLogoutProfileName(pc, []string{"explicit"}, "personal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "explicit" {
		t.Fatalf("resolveLogoutProfileName() = %q, want %q", got, "explicit")
	}
}

func TestResolveLogoutProfileName_NoProfilesConfigured(t *testing.T) {
	pc := &config.ProfileConfig{Profiles: map[string]config.Config{}}

	_, err := resolveLogoutProfileName(pc, nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != noProfilesConfiguredMessage {
		t.Fatalf("error = %q, want %q", got, noProfilesConfiguredMessage)
	}
}

func TestResolveLogoutProfileName_NoActiveProfile(t *testing.T) {
	pc := &config.ProfileConfig{
		Profiles: map[string]config.Config{
			"work":     {Server: "https://work.example.com"},
			"personal": {Server: "https://personal.example.com"},
		},
	}

	_, err := resolveLogoutProfileName(pc, nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != noActiveProfileMessage {
		t.Fatalf("error = %q, want %q", got, noActiveProfileMessage)
	}
}

func TestLogout_NoProfilesConfiguredWarns(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("profiles: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdLogout(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for empty profiles, got: %v", err)
	}

	output := f.IOStreams.ErrOut.(*strings.Builder).String()
	if !strings.Contains(output, noProfilesConfiguredMessage) {
		t.Fatalf("expected warning %q, got:\n%s", noProfilesConfiguredMessage, output)
	}
}

func TestLogout_NoProfilesConfigured_IgnoresJSONDefault(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("profiles: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath:   cfgPath,
		OutputFormat: "json",
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdLogout(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for empty profiles, got: %v", err)
	}

	if out := f.IOStreams.Out.(*strings.Builder).String(); out != "" {
		t.Fatalf("expected no stdout output, got %q", out)
	}
	if errOut := f.IOStreams.ErrOut.(*strings.Builder).String(); !strings.Contains(errOut, noProfilesConfiguredMessage) {
		t.Fatalf("expected warning %q, got:\n%s", noProfilesConfiguredMessage, errOut)
	}
}
