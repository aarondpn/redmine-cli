package cmdutil

import (
	"context"

	"github.com/spf13/cobra"
)

// Context returns the context a command runs under: the one main passes to
// ExecuteContext, which is cancelled on the first SIGINT/SIGTERM. A command
// whose RunE is invoked without Execute has none, so this falls back to
// context.Background().
func Context(cmd *cobra.Command) context.Context {
	if cmd != nil {
		if ctx := cmd.Context(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}
