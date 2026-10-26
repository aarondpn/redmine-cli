package cmdutil

import (
	"fmt"
)

// Warn prints msg as a warning to stderr, once per distinct message for the
// lifetime of the Factory. It is safe for concurrent use.
func (f *Factory) Warn(msg string) {
	f.warnMu.Lock()
	defer f.warnMu.Unlock()
	if f.warned == nil {
		f.warned = make(map[string]bool)
	}
	if f.warned[msg] {
		return
	}
	f.warned[msg] = true
	if f.IOStreams != nil && f.IOStreams.ErrOut != nil {
		fmt.Fprintf(f.IOStreams.ErrOut, "warning: %s\n", msg)
	}
}
