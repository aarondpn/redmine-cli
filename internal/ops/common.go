package ops

// DefaultListLimit caps list-style operations when the caller omits a limit so
// MCP clients do not accidentally pull large result sets into context.
const DefaultListLimit = 50

// NoLimit is the sentinel callers pass when they want to bypass the safety cap
// and fetch every result. The CLI translates `--limit 0` into this value before
// invoking ops; MCP callers should never use it.
const NoLimit = -1

// ListLimit returns the effective limit for a list operation.
//
// Negative input (NoLimit) is translated to the API client's "0 = unlimited"
// convention. Zero applies the MCP-safety default. Positive values pass
// through unchanged.
func ListLimit(requested int) int {
	if requested < 0 {
		return 0
	}
	if requested == 0 {
		return DefaultListLimit
	}
	return requested
}

// MessageResult is a simple acknowledgement payload for mutating operations.
type MessageResult struct {
	Message string `json:"message"`
}

// MCPMessage exposes the human-readable message used by the MCP wrapper.
func (m MessageResult) MCPMessage() string {
	return m.Message
}
