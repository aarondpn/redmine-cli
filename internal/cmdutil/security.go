package cmdutil

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// StrictModeError is returned by SecurityCheck when strict mode turns a
// security warning into a hard failure.
type StrictModeError struct {
	Msg string
}

func (e *StrictModeError) Error() string {
	return fmt.Sprintf("%s (refusing because strict mode is enabled; unset strict / REDMINE_STRICT to downgrade this to a warning)", e.Msg)
}

// SecurityCheck reports a security-relevant condition. With cfg.Strict it
// returns a *StrictModeError so the command aborts; otherwise it prints the
// message as a warning to stderr (once per distinct message) and returns nil.
// A nil cfg is treated as non-strict.
func (f *Factory) SecurityCheck(cfg *config.Config, msg string) error {
	if cfg != nil && cfg.Strict {
		return &StrictModeError{Msg: msg}
	}
	if f.securityWarned == nil {
		f.securityWarned = make(map[string]bool)
	}
	if f.securityWarned[msg] {
		return nil
	}
	f.securityWarned[msg] = true
	if f.IOStreams != nil && f.IOStreams.ErrOut != nil {
		fmt.Fprintf(f.IOStreams.ErrOut, "warning: %s\n", msg)
	}
	return nil
}
