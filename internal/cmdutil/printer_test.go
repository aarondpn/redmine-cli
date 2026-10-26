package cmdutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func TestPrinterCSVFormulaWarning(t *testing.T) {
	rows := [][]string{{"1", "=HYPERLINK(\"http://evil.example\")", "-1.5"}}
	f := testFactoryWithConfig(t, "")

	// Two exports in one run still produce a single warning.
	f.Printer(output.FormatCSV).CSV([]string{"ID", "SUBJECT", "HOURS"}, rows)
	out := f.IOStreams.Out.(*bytes.Buffer)
	if want := "ID,SUBJECT,HOURS\n1,\"=HYPERLINK(\"\"http://evil.example\"\")\",-1.5\n"; out.String() != want {
		t.Errorf("CSV output = %q, want %q", out.String(), want)
	}
	f.Printer(output.FormatCSV).CSV([]string{"ID", "SUBJECT", "HOURS"}, rows)

	errOut := f.IOStreams.ErrOut.(*bytes.Buffer).String()
	if got := strings.Count(errOut, "warning: "+output.CSVFormulaWarning); got != 1 {
		t.Errorf("got %d formula warnings, want 1; stderr: %q", got, errOut)
	}
}

// Ordinary subjects such as "- fix typo" trigger the formula warning, so a
// list with many of them, exported page by page, must still print the
// warning exactly once, on a single line.
func TestPrinterCSVFormulaWarningOncePerCommand(t *testing.T) {
	f := testFactoryWithConfig(t, "")
	rows := [][]string{{"1", "- fix typo"}, {"2", "- update docs"}, {"3", "+1 from QA"}}
	for range 3 {
		f.Printer(output.FormatCSV).CSV([]string{"ID", "SUBJECT"}, rows)
	}
	if got, want := f.IOStreams.ErrOut.(*bytes.Buffer).String(), "warning: "+output.CSVFormulaWarning+"\n"; got != want {
		t.Errorf("stderr = %q, want exactly one single-line warning %q", got, want)
	}
}

func TestPrinterJQEnv(t *testing.T) {
	t.Setenv("REDMINE_JQ_TEST_SECRET", "s3cret")

	tests := []struct {
		name  string
		noEnv string
		want  string
	}{
		{name: "default keeps env", want: "s3cret\n"},
		{name: "REDMINE_JQ_NO_ENV hides env", noEnv: "1", want: "null\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(output.JQNoEnvVar, tt.noEnv)
			f := testFactoryWithConfig(t, "")
			q, err := output.CompileJQ("$ENV.REDMINE_JQ_TEST_SECRET")
			if err != nil {
				t.Fatal(err)
			}
			f.JQ = q

			f.Printer(output.FormatJSON).JSON(map[string]any{})
			if got := f.IOStreams.Out.(*bytes.Buffer).String(); got != tt.want {
				t.Errorf("jq output = %q, want %q", got, tt.want)
			}
		})
	}
}
