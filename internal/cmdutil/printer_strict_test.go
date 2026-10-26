package cmdutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func TestPrinterCSVFormulaGuard(t *testing.T) {
	rows := [][]string{{"1", "=HYPERLINK(\"http://evil.example\")", "-1.5"}}

	tests := []struct {
		name     string
		config   string
		wantOut  string
		wantWarn bool
	}{
		{
			name:     "default warns once and keeps output verbatim",
			config:   "",
			wantOut:  "ID,SUBJECT,HOURS\n1,\"=HYPERLINK(\"\"http://evil.example\"\")\",-1.5\n",
			wantWarn: true,
		},
		{
			name:    "strict neutralizes formula cells",
			config:  "strict: true\n",
			wantOut: "ID,SUBJECT,HOURS\n1,\"'=HYPERLINK(\"\"http://evil.example\"\")\",-1.5\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDMINE_STRICT", "")
			f := testFactoryWithConfig(t, tt.config)

			// Two exports in one run still produce a single warning.
			f.Printer(output.FormatCSV).CSV([]string{"ID", "SUBJECT", "HOURS"}, rows)
			out := f.IOStreams.Out.(*bytes.Buffer)
			if out.String() != tt.wantOut {
				t.Errorf("CSV output = %q, want %q", out.String(), tt.wantOut)
			}
			f.Printer(output.FormatCSV).CSV([]string{"ID", "SUBJECT", "HOURS"}, rows)

			errOut := f.IOStreams.ErrOut.(*bytes.Buffer).String()
			wantCount := 0
			if tt.wantWarn {
				wantCount = 1
			}
			if got := strings.Count(errOut, "warning: "+output.CSVFormulaWarning); got != wantCount {
				t.Errorf("got %d formula warnings, want %d; stderr: %q", got, wantCount, errOut)
			}
		})
	}
}

// Ordinary subjects such as "- fix typo" trigger the formula warning, so a
// list with many of them, exported page by page, must still print the
// warning exactly once, on a single line.
func TestPrinterCSVFormulaWarningOncePerCommand(t *testing.T) {
	t.Setenv("REDMINE_STRICT", "")
	f := testFactoryWithConfig(t, "")
	rows := [][]string{{"1", "- fix typo"}, {"2", "- update docs"}, {"3", "+1 from QA"}}
	for range 3 {
		f.Printer(output.FormatCSV).CSV([]string{"ID", "SUBJECT"}, rows)
	}
	if got, want := f.IOStreams.ErrOut.(*bytes.Buffer).String(), "warning: "+output.CSVFormulaWarning+"\n"; got != want {
		t.Errorf("stderr = %q, want exactly one single-line warning %q", got, want)
	}
}

func TestPrinterJQEnvHiddenInStrictMode(t *testing.T) {
	t.Setenv("REDMINE_JQ_TEST_SECRET", "s3cret")
	t.Setenv(output.JQNoEnvVar, "")

	tests := []struct {
		name   string
		config string
		want   string
	}{
		{name: "default keeps env", config: "", want: "s3cret\n"},
		{name: "strict hides env", config: "strict: true\n", want: "null\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDMINE_STRICT", "")
			f := testFactoryWithConfig(t, tt.config)
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
