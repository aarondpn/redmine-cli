package ops

// DefaultListLimit caps list-style operations when the caller omits a limit so
// MCP clients do not accidentally pull large result sets into context.
const DefaultListLimit = 50

// ListLimit returns the effective limit for a list operation.
func ListLimit(requested int) int {
	if requested <= 0 {
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
