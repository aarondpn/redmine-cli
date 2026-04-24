package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

// registerTools wires every tool onto the server. Mutating tools are only
// registered when opts.EnableWrites is true, so they never appear in tools/list
// in read-only mode.
func registerTools(s *mcp.Server, client *api.Client, opts Options) {
	registerIssueTools(s, client, opts)
	registerProjectTools(s, client, opts)
	registerTimeEntryTools(s, client, opts)
	registerUserTools(s, client, opts)
	registerSearchTools(s, client)
	registerMetaTools(s, client, opts)
	registerWikiTools(s, client, opts)
	registerMembershipTools(s, client, opts)
}

// defaultListLimit caps list_* tools when the caller omits a limit so a
// language model cannot accidentally pull thousands of rows into its context.
const defaultListLimit = 50

// listLimit returns the effective limit for a list_* tool argument.
func listLimit(requested int) int {
	if requested <= 0 {
		return defaultListLimit
	}
	return requested
}
