package cmdutil

import (
	"bytes"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

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
