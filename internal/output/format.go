package output

import (
	"os"

	"golang.org/x/term"
)

// Format constants for output modes.
const (
	FormatTable = "table"
	FormatWide  = "wide"
	FormatJSON  = "json"
	FormatCSV   = "csv"
)

// IsTerminal checks if stdout is a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
