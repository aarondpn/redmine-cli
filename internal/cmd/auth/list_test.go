package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

func TestList_JSONMarksSoleProfileActive(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `profiles:
  only:
    server: https://only.example.com
    auth_method: apikey
    api_key: only-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
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

	cmd := NewCmdList(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var profiles []struct {
		Name   string `json:"name"`
		Server string `json:"server"`
		Active bool   `json:"active"`
	}
	if err := json.Unmarshal([]byte(f.IOStreams.Out.(*strings.Builder).String()), &profiles); err != nil {
		t.Fatalf("expected valid JSON output, got: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "only" {
		t.Fatalf("name = %q, want %q", profiles[0].Name, "only")
	}
	if !profiles[0].Active {
		t.Fatalf("expected sole profile to be marked active, got %+v", profiles[0])
	}
}
