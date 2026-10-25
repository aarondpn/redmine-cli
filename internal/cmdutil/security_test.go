package cmdutil

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

func TestSecurityCheck_WarnsOnceWhenNotStrict(t *testing.T) {
	var errOut bytes.Buffer
	f := &Factory{IOStreams: &IOStreams{ErrOut: &errOut}}
	cfg := &config.Config{}

	for i := 0; i < 2; i++ {
		if err := f.SecurityCheck(cfg, "plaintext server"); err != nil {
			t.Fatalf("non-strict check returned error: %v", err)
		}
	}
	if got := strings.Count(errOut.String(), "warning: plaintext server"); got != 1 {
		t.Fatalf("expected exactly one warning, got %d in %q", got, errOut.String())
	}
}

func TestSecurityCheck_StrictReturnsError(t *testing.T) {
	var errOut bytes.Buffer
	f := &Factory{IOStreams: &IOStreams{ErrOut: &errOut}}

	err := f.SecurityCheck(&config.Config{Strict: true}, "plaintext server")
	var strictErr *StrictModeError
	if !errors.As(err, &strictErr) {
		t.Fatalf("expected *StrictModeError, got %v", err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("strict mode must not also print a warning, got %q", errOut.String())
	}
}
