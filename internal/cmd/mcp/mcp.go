// Package mcp wires the Model Context Protocol server subcommand into the
// Redmine CLI.
package mcp

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// NewCmdMCP creates the parent `mcp` command.
func NewCmdMCP(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol server",
		Long: "Expose the Redmine CLI as a Model Context Protocol (MCP) server " +
			"so AI assistants such as Claude Desktop, Claude Code, and Cursor " +
			"can drive Redmine through the same profile-backed authentication.",
	}
	cmd.AddCommand(newCmdServe(f))
	return cmd
}
