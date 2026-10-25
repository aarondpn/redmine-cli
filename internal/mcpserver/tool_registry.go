package mcpserver

import (
	"context"
	"reflect"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

type messageResult interface {
	MCPMessage() string
}

type toolSpec[In, Out any] struct {
	Name        string
	Description string
	Category    Group
	Writes      bool
	Call        func(context.Context, *api.Client, In) (Out, error)
}

func registerToolSpec[In, Out any](srv *mcp.Server, client *api.Client, opts Options, spec toolSpec[In, Out]) {
	if spec.Writes && !opts.EnableWrites {
		return
	}
	if spec.Category != "" && !opts.groupEnabled(spec.Category) {
		return
	}
	if !opts.toolEnabled(spec.Name) {
		return
	}

	mcp.AddTool(srv, &mcp.Tool{
		Name:        spec.Name,
		Description: spec.Description,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		clampMCPInput(&input)
		output, err := spec.Call(ctx, client, input)
		if err != nil {
			return toolErrFromError[Out](err)
		}
		return toolResult(output)
	})
}

// clampMCPInput zeroes any negative `Limit` or `Offset` int field on the
// decoded tool input. The ops layer's NoLimit sentinel (-1) means "fetch
// every page"; without this clamp an MCP client could send `{"limit": -1}`
// to bypass the safety cap that protects the model context from large list
// responses.
func clampMCPInput[T any](in *T) {
	v := reflect.ValueOf(in).Elem()
	if v.Kind() != reflect.Struct {
		return
	}
	for _, name := range [...]string{"Limit", "Offset"} {
		f := v.FieldByName(name)
		if !f.IsValid() || !f.CanSet() || f.Kind() != reflect.Int {
			continue
		}
		if f.Int() < 0 {
			f.SetInt(0)
		}
	}
}

// emptyNilSlices replaces nil slice fields of a struct result with empty
// slices, so list tools encode "no results" as [] instead of null. It mirrors
// output.RenderJSON on the CLI side. Fields tagged omitempty/omitzero are left
// alone: nil there means "not fetched" and the key is omitted, not null.
func emptyNilSlices[T any](v *T) {
	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Kind() != reflect.Slice || !f.IsNil() || !f.CanSet() {
			continue
		}
		tag := rt.Field(i).Tag.Get("json")
		if tag == "-" || strings.Contains(tag, ",omitempty") || strings.Contains(tag, ",omitzero") {
			continue
		}
		f.Set(reflect.MakeSlice(f.Type(), 0, 0))
	}
}

// toolResult is the single exit point for successful tool calls. Secrets are
// redacted here, before both the text content and the structured value are
// produced, so no tool can return an API key to the client.
func toolResult[T any](v T) (*mcp.CallToolResult, T, error) {
	redactSecrets(&v)
	emptyNilSlices(&v)
	asMessage, ok := any(v).(messageResult)
	if ok {
		return toolOKMsgWithValue(v, asMessage.MCPMessage())
	}
	return toolOK(v)
}
