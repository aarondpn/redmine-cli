package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

//go:generate go run ../../cmd/mcpgen

// registerTools wires every tool onto the server: the generated tools, then
// the extension tools. Mutating tools are only registered when
// opts.EnableWrites is true, so they never appear in tools/list in read-only
// mode.
func registerTools(s *mcp.Server, client *api.Client, opts Options) {
	registerGeneratedTools(s, client, opts)
	registerExtensionTools(s, opts)
}
