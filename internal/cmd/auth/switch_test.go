package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func TestSwitch_NoProfilesConfiguredWarns(t *testing.T) {
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

	cmd := NewCmdSwitch(f)
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

func TestSwitch_NoProfilesConfigured_JSONEnvelope(t *testing.T) {
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

	cmd := NewCmdSwitch(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for empty profiles, got: %v", err)
	}

	var env struct {
		Ok      bool   `json:"ok"`
		Action  string `json:"action"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(f.IOStreams.Out.(*strings.Builder).String()), &env); err != nil {
		t.Fatalf("expected JSON output, got: %v", err)
	}
	if env.Ok {
		t.Fatalf("expected ok=false envelope, got %+v", env)
	}
	if env.Action != "switched" || env.Message != noProfilesConfiguredMessage {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}
