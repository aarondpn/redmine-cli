package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func TestRootCmdGeneratesCompletions(t *testing.T) {
	root := NewRootCmd("test")
	for _, shell := range []struct {
		name string
		gen  func() error
	}{
		{"bash", func() error { return root.GenBashCompletion(io.Discard) }},
		{"zsh", func() error { return root.GenZshCompletion(io.Discard) }},
		{"fish", func() error { return root.GenFishCompletion(io.Discard, true) }},
		{"powershell", func() error { return root.GenPowerShellCompletion(io.Discard) }},
	} {
		t.Run(shell.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("completion generation panicked (likely a flag shorthand collision): %v", r)
				}
			}()
			if err := shell.gen(); err != nil {
				t.Fatalf("completion generation failed: %v", err)
			}
		})
	}
}

func TestConfigCommandShowsSoleProfileName(t *testing.T) {
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
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := newCmdConfig(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := f.IOStreams.Out.(*strings.Builder).String()
	if !strings.Contains(output, "only") {
		t.Errorf("expected config output to contain the sole profile name, got:\n%s", output)
	}
	if !strings.Contains(output, "https://only.example.com") {
		t.Errorf("expected config output to contain the sole profile server, got:\n%s", output)
	}
}
