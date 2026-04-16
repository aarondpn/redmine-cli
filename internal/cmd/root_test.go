package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/spf13/cobra"
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

func TestRootCmd_PersistentOutputFlag_InheritedByLeafCommands(t *testing.T) {
	root := NewRootCmd("test")
	optedOut := map[string]bool{
		"redmine auth login":    true,
		"redmine auth logout":   true,
		"redmine auth switch":   true,
		"redmine issues browse": true,
		"redmine search browse": true,
	}

	// The persistent flag must be registered on the root.
	if root.PersistentFlags().Lookup("output") == nil {
		t.Fatal("root command missing persistent --output flag")
	}

	// Walk all leaf commands and verify they can see the inherited flag.
	var missing []string
	walk(root, func(c *cobra.Command) {
		if c.Runnable() && !c.Hidden && !optedOut[c.CommandPath()] {
			if c.Flags().Lookup("output") == nil && c.InheritedFlags().Lookup("output") == nil {
				missing = append(missing, c.CommandPath())
			}
		}
	})
	if len(missing) > 0 {
		t.Fatalf("leaf commands missing --output: %v", missing)
	}
}

func TestRootCmd_InteractiveCommandsRejectOutputFlag(t *testing.T) {
	for _, args := range [][]string{
		{"auth", "login", "--output", "json"},
		{"auth", "logout", "--output", "json"},
		{"auth", "switch", "--output", "json"},
		{"issues", "browse", "--output", "json"},
		{"search", "browse", "--output", "json"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			root := NewRootCmd("test")
			root.SetArgs(args)

			err := root.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "--output is not supported") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRootCmdWithFactory_ReturnsFactoryAndSetsOutputFormat(t *testing.T) {
	root, factory := NewRootCmdWithFactory("test")
	if factory == nil {
		t.Fatal("factory must not be nil")
	}

	// Execute PersistentPreRunE with -o json to confirm factory wiring.
	root.SetArgs([]string{"--output", "json", "config"})
	_ = root.PersistentFlags().Set("output", "json")

	if err := root.PersistentPreRunE(root, []string{}); err != nil {
		t.Fatalf("PersistentPreRunE: %v", err)
	}
	if factory.OutputFormat != "json" {
		t.Errorf("factory.OutputFormat = %q, want %q", factory.OutputFormat, "json")
	}
}

func walk(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, child := range c.Commands() {
		walk(child, fn)
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
