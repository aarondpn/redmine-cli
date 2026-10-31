package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
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

// toolErrFromError returns a tool-level error for both API failures and local
// validation/runtime failures from the ops layer.
func toolErrFromError[T any](err error) (*mcp.CallToolResult, T, error) {
	var he *ops.HintedError
	if errors.As(err, &he) {
		// Keep the remediation hint next to the described API failure.
		return toolErr[T](describeAPIError(he.Err) + " (" + he.Hint + ")")
	}
	var (
		ae *api.APIError
		pe *api.InvalidPathSegmentError
	)
	if errors.As(err, &ae) || errors.As(err, &pe) || errors.Is(err, oauth.ErrLoginRequired) {
		return toolErrFromAPI[T](err)
	}
	return toolErr[T](err.Error())
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

func toolOKMsgWithValue[T any](v T, msg string) (*mcp.CallToolResult, T, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, v, nil
}

// describeAPIError converts an error from the api package to a user-facing
// message.
func describeAPIError(err error) string {
	var pe *api.InvalidPathSegmentError
	if errors.As(err, &pe) {
		return fmt.Sprintf("Invalid argument: %s %q %s. The request was not sent to Redmine; pass the value as Redmine shows it (for a project, its identifier or numeric ID).", pe.Kind, pe.Value, pe.Reason)
	}
	if errors.Is(err, oauth.ErrLoginRequired) {
		return "The OAuth login expired or was revoked. The user must run 'redmine auth login --oauth' in a terminal; the MCP server picks up the new login without a restart."
	}
	var ae *api.APIError
	if errors.As(err, &ae) {
		switch {
		case ae.IsAuthError():
			return "Authentication failed: Redmine did not accept the credentials (invalid, revoked or missing API key, or a locked account). The user must fix them outside the chat: run 'redmine auth login' in a terminal (add --name <profile> to update a specific profile), or update REDMINE_API_KEY in the MCP host config, then restart the MCP server."
		case ae.IsForbidden():
			// Redmine answers 403 both for a missing permission and for every
			// login-requiring request while the REST API is disabled.
			return "Forbidden: either the account lacks the permission for this operation, or the REST API is disabled. If other tools work, it is a missing permission; don't retry with other credentials. If every tool is denied, an administrator must enable Administration > Settings > API > Enable REST web service."
		case ae.IsNotFound():
			return "Resource not found: " + ae.URL
		case ae.IsValidationError():
			if len(ae.Errors) > 0 {
				return "Validation error: " + strings.Join(ae.Errors, "; ")
			}
			return "Validation error (HTTP 422)"
		case ae.IsConflict():
			if len(ae.Errors) > 0 {
				return "Conflict: " + strings.Join(ae.Errors, "; ")
			}
			return "Conflict (HTTP 409): the resource was modified since it was last fetched."
		default:
			if len(ae.Errors) > 0 {
				return fmt.Sprintf("Redmine API error %d: %s", ae.StatusCode, strings.Join(ae.Errors, "; "))
			}
			return fmt.Sprintf("Redmine API error %d for %s", ae.StatusCode, ae.URL)
		}
	}
	return err.Error()
}
