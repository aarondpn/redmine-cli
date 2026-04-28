package mcp

import (
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

func TestNewCmdMCP_HasServe(t *testing.T) {
	cmd := NewCmdMCP(cmdutil.NewFactory())
	if cmd.Use != "mcp" {
		t.Errorf("Use = %q, want mcp", cmd.Use)
	}
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "serve" {
			found = true
		}
	}
	if !found {
		t.Fatal("mcp serve subcommand not registered")
	}
}

func TestServeFlags_DefaultsToReadOnly(t *testing.T) {
	cmd := newCmdServe(cmdutil.NewFactory())

	enable, err := cmd.Flags().GetBool("enable-writes")
	if err != nil {
		t.Fatalf("GetBool: %v", err)
	}
	if enable {
		t.Error("--enable-writes defaulted to true; expected false")
	}

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if name != "redmine-cli" {
		t.Errorf("--name default = %q, want redmine-cli", name)
	}

	httpAddr, err := cmd.Flags().GetString("http")
	if err != nil {
		t.Fatalf("GetString(http): %v", err)
	}
	if httpAddr != "" {
		t.Errorf("--http default = %q, want empty string", httpAddr)
	}
}
