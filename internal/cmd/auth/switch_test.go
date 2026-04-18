package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
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

func TestSwitch_NoProfilesConfigured_IgnoresJSONDefault(t *testing.T) {
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

	if out := f.IOStreams.Out.(*strings.Builder).String(); out != "" {
		t.Fatalf("expected no stdout output, got %q", out)
	}
	if errOut := f.IOStreams.ErrOut.(*strings.Builder).String(); !strings.Contains(errOut, noProfilesConfiguredMessage) {
		t.Fatalf("expected warning %q, got:\n%s", noProfilesConfiguredMessage, errOut)
	}
}

func TestSwitch_WithNamedProfile_AllowsJSONOutput(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `profiles:
  demo:
    server: https://redmine.example.com
    auth_method: apikey
    api_key: test
active_profile: demo
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
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
	cmd.SetArgs([]string{"demo", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var env struct {
		Ok       bool   `json:"ok"`
		Action   string `json:"action"`
		Resource string `json:"resource"`
		ID       string `json:"id"`
	}
	if err := json.Unmarshal([]byte(f.IOStreams.Out.(*strings.Builder).String()), &env); err != nil {
		t.Fatalf("expected JSON output, got: %v", err)
	}
	if !env.Ok || env.Action != "switched" || env.Resource != "profile" || env.ID != "demo" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if errOut := f.IOStreams.ErrOut.(*strings.Builder).String(); errOut != "" {
		t.Fatalf("expected empty stderr, got %q", errOut)
	}
}
