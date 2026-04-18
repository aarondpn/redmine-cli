package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

// BuildServer constructs an MCP server wired to the given Redmine API client.
// Tools that mutate remote state are only registered when opts.EnableWrites is
// true.
func BuildServer(client *api.Client, opts Options) *mcp.Server {
	name := opts.Name
	if name == "" {
		name = "redmine-cli"
	}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Version: opts.Version,
	}, nil)

	registerTools(srv, client, opts)
	registerResources(srv, client)

	return srv
}
