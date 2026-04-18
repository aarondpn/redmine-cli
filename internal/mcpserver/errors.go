package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

// toolErr returns an error-flavored CallToolResult so the MCP client reports
// a tool error (not a protocol error).
func toolErr[T any](msg string) (*mcp.CallToolResult, T, error) {
	var zero T
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, zero, nil
}

// toolErrFromAPI classifies an error returned by the Redmine API client and
// wraps it into an MCP tool-error result with a human-readable message.
func toolErrFromAPI[T any](err error) (*mcp.CallToolResult, T, error) {
	return toolErr[T](describeAPIError(err))
}

// toolOK returns a success CallToolResult that carries the marshalled value
// both as TextContent (for clients that only consume content) and as the
// structured return value (for clients that understand structuredContent).
func toolOK[T any](v T) (*mcp.CallToolResult, T, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return toolErr[T]("failed to encode response: " + err.Error())
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}, v, nil
}

// toolOKMsg returns a success CallToolResult that only carries a human text
// confirmation (used by write tools that have no meaningful return value).
func toolOKMsg(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, nil, nil
}

// describeAPIError converts an error from the api package to a user-facing
// message.
func describeAPIError(err error) string {
	var ae *api.APIError
	if errors.As(err, &ae) {
		switch {
		case ae.IsAuthError():
			return "Authentication failed. Check the API key or run 'redmine auth login --profile <name>'."
		case ae.IsForbidden():
			return "Forbidden. The account does not have permission for this operation."
		case ae.IsNotFound():
			return "Resource not found: " + ae.URL
		case ae.IsValidationError():
			if len(ae.Errors) > 0 {
				return "Validation error: " + strings.Join(ae.Errors, "; ")
			}
			return "Validation error (HTTP 422)"
		default:
			if len(ae.Errors) > 0 {
				return fmt.Sprintf("Redmine API error %d: %s", ae.StatusCode, strings.Join(ae.Errors, "; "))
			}
			return fmt.Sprintf("Redmine API error %d for %s", ae.StatusCode, ae.URL)
		}
	}
	return err.Error()
}
