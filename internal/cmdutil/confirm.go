package cmdutil

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ConfirmAction prompts the user to confirm a destructive action.
// Returns true if the user confirms with "y" or "yes".
// If in is not interactive or the user declines, returns false.
func ConfirmAction(in io.Reader, errOut io.Writer, message string) bool {
	fmt.Fprintf(errOut, "%s [y/N]: ", message)
	reader := bufio.NewReader(in)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
