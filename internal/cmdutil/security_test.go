package cmdutil

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestWarn_Once(t *testing.T) {
	var errOut bytes.Buffer
	f := &Factory{IOStreams: &IOStreams{ErrOut: &errOut}}

	for i := 0; i < 2; i++ {
		f.Warn("plaintext server")
	}
	if got := strings.Count(errOut.String(), "warning: plaintext server"); got != 1 {
		t.Fatalf("expected exactly one warning, got %d in %q", got, errOut.String())
	}
}

func TestWarn_ConcurrentOnce(t *testing.T) {
	var errOut bytes.Buffer
	f := &Factory{IOStreams: &IOStreams{ErrOut: &errOut}}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f.Warn("shared")
			f.Warn(fmt.Sprintf("distinct %d", i))
		}(i)
	}
	wg.Wait()

	out := errOut.String()
	if got := strings.Count(out, "warning: shared\n"); got != 1 {
		t.Fatalf("shared warning printed %d times, want 1:\n%s", got, out)
	}
	if got := strings.Count(out, "warning: distinct "); got != 64 {
		t.Fatalf("distinct warnings printed %d times, want 64:\n%s", got, out)
	}
}
