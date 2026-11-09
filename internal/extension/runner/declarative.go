package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// RunCommand runs the declarative command c of the extension executable at
// path the way both surfaces do: no arguments, input (already validated,
// defaults applied) as JSON on stdin, the command's timeout, and env as the
// complete environment (see Env.Environ). stderr, when set, receives the
// extension's stderr live.
func RunCommand(ctx context.Context, path string, c *spec.Command, input map[string]any, env []string, stderr io.Writer) (*Result, error) {
	if input == nil {
		input = map[string]any{}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encoding input: %w", err)
	}
	return Run(ctx, Request{
		Path:    path,
		Env:     env,
		Stdin:   data,
		Stderr:  stderr,
		Timeout: c.TimeoutDuration(),
	})
}
