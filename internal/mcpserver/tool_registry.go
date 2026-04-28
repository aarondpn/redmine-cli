package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

type messageResult interface {
	MCPMessage() string
}

type toolSpec[In, Out any] struct {
	Name        string
	Description string
	Writes      bool
	Call        func(context.Context, *api.Client, In) (Out, error)
}

func registerToolSpec[In, Out any](srv *mcp.Server, client *api.Client, opts Options, spec toolSpec[In, Out]) {
	if spec.Writes && !opts.EnableWrites {
		return
	}

	mcp.AddTool(srv, &mcp.Tool{
		Name:        spec.Name,
		Description: spec.Description,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		output, err := spec.Call(ctx, client, input)
		if err != nil {
			return toolErrFromError[Out](err)
		}
		return toolResult(output)
	})
}

func toolResult[T any](v T) (*mcp.CallToolResult, T, error) {
	asMessage, ok := any(v).(messageResult)
	if ok {
		return toolOKMsgWithValue(v, asMessage.MCPMessage())
	}
	return toolOK(v)
}
