// Package mcpserver implements a Model Context Protocol server that exposes
// the redmine-cli API surface to MCP-compatible hosts (Claude Desktop, Claude
// Code, Cursor, etc.).
package mcpserver

// Options controls how the MCP server is built.
type Options struct {
	// EnableWrites gates registration of mutating tools (create/update/delete
	// and similar). When false, those tools are not registered and therefore
	// never appear in tools/list.
	EnableWrites bool

	// Name is the server name advertised in the MCP initialize response.
	Name string

	// Version is the server version advertised in the MCP initialize response.
	Version string
}
