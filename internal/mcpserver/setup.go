package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SetupToolName is the only tool the unconfigured server registers.
const SetupToolName = "setup_status"

// SetupStatus is the result of the setup_status tool.
type SetupStatus struct {
	Configured   bool   `json:"configured" jsonschema:"always false: no Redmine connection is configured"`
	Problem      string `json:"problem" jsonschema:"why the server could not connect to Redmine"`
	Instructions string `json:"instructions" jsonschema:"what the user has to do to fix it"`
}

// SetupInstructions returns the text shown to the agent when the server runs
// without a usable Redmine configuration. problem is the underlying error.
// The text never asks for a secret: the user enters it outside the model's
// context (terminal prompt, stdin, or the MCP host's env config).
func SetupInstructions(problem string) string {
	var b strings.Builder
	b.WriteString("The Redmine MCP server is running but not connected to a Redmine instance, so no Redmine tools are available. ")
	b.WriteString("Tell the user to set it up, then restart or reconnect the MCP server in their MCP host:\n")
	b.WriteString("1. Run `redmine auth login` in a terminal (interactive), or non-interactively `redmine auth login --server https://redmine.example.com --api-key-file -` and pipe the API key to its standard input.\n")
	b.WriteString("2. Or set REDMINE_SERVER and REDMINE_API_KEY in the env block of this MCP server in the host's configuration.\n")
	b.WriteString("3. If several profiles are already configured but none is active, run `redmine auth switch` (or add `--profile <name>` to the MCP server args in the host configuration) to pick one instead of logging in again.\n")
	b.WriteString("Never ask the user to paste an API key, password or token into the chat, and never pass one to a tool; the user must enter it themselves in the terminal or the host configuration.\n")
	if problem != "" {
		b.WriteString("Underlying problem: " + problem + "\n")
	}
	return b.String()
}

// BuildSetupServer constructs an MCP server for the case where no usable
// Redmine configuration exists. It registers no Redmine tools or resources,
// only the read-only setup_status tool, and advertises SetupInstructions as
// the server instructions. problem is the underlying configuration error.
func BuildSetupServer(problem string, opts Options) *mcp.Server {
	name := opts.Name
	if name == "" {
		name = "redmine-cli"
	}
	instructions := SetupInstructions(problem)
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    name,
		Version: opts.Version,
	}, &mcp.ServerOptions{Instructions: instructions})

	// No input: the tool must never be a channel for credentials.
	type noInput struct{}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        SetupToolName,
		Description: "Explain why the Redmine MCP server is not connected and what the user has to do to fix it. Takes no arguments.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(context.Context, *mcp.CallToolRequest, noInput) (*mcp.CallToolResult, SetupStatus, error) {
		return toolOK(SetupStatus{Configured: false, Problem: problem, Instructions: instructions})
	})
	return srv
}
