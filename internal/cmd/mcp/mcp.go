// Package mcp wires the Model Context Protocol server subcommand into the
// Redmine CLI.
package mcp

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
)

// Options configures the mcp commands.
type Options struct {
	// Extensions returns the installed extensions whose declarative
	// commands become MCP tools. nil means none: tests build the root
	// through NewRootCmd and must not depend on the extensions installed
	// on the developer's machine.
	Extensions func() ([]*extmgr.Extension, error)
}

// NewCmdMCP creates the parent `mcp` command.
func NewCmdMCP(f *cmdutil.Factory, opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol server",
		Long: "Expose the Redmine CLI as a Model Context Protocol (MCP) server " +
			"so AI assistants such as Claude Desktop, Claude Code, and Cursor " +
			"can drive Redmine through the same profile-backed authentication.",
	}
	cmd.AddCommand(newCmdServe(f, opts))
	cmd.AddCommand(newCmdTools(f, opts))
	return cmd
}
