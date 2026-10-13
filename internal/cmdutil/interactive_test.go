package cmdutil

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newInteractiveTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "browse"}
	cmd.Flags().StringP("output", "o", "", "")
	cmd.Flags().StringP("jq", "q", "", "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	return cmd
}

// TestPrepareInteractiveCommand_RejectionKeepsRequestedFormat pins that a
// rejected --jq / --output leaves the factory's format alone, so the error
// path renders the JSON envelope the caller asked for instead of text.
func TestPrepareInteractiveCommand_RejectionKeepsRequestedFormat(t *testing.T) {
	for _, args := range [][]string{{"--jq", "."}, {"-o", "json"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			q, err := output.CompileJQ(".")
			if err != nil {
				t.Fatal(err)
			}
			f := &Factory{OutputFormat: output.FormatJSON, JQ: q}
			err = PrepareInteractiveCommand(newInteractiveTestCmd(t, args...), f)
			if err == nil || !strings.Contains(err.Error(), "is not supported for browse") {
				t.Fatalf("err = %v, want rejection", err)
			}
			if f.OutputFormat != output.FormatJSON {
				t.Errorf("OutputFormat = %q, want %q kept for the error envelope", f.OutputFormat, output.FormatJSON)
			}
		})
	}
}

func TestPrepareInteractiveCommand_ForcesTableWithoutFlags(t *testing.T) {
	f := &Factory{OutputFormat: output.FormatJSON}
	if err := PrepareInteractiveCommand(newInteractiveTestCmd(t), f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.OutputFormat != output.FormatTable || f.JQ != nil {
		t.Errorf("OutputFormat = %q, JQ = %v; want table and no jq", f.OutputFormat, f.JQ)
	}
}
