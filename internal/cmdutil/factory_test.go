package cmdutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPrinterUsesConfiguredOutputFormatByDefault(t *testing.T) {
	f := testFactoryWithConfig(t, "output_format: json\n")

	printer := f.Printer("")

	if got := printer.Format(); got != "json" {
		t.Fatalf("Format() = %q, want %q", got, "json")
	}
}

func TestPrinterExplicitFormatOverridesConfig(t *testing.T) {
	f := testFactoryWithConfig(t, "output_format: json\n")

	printer := f.Printer("csv")

	if got := printer.Format(); got != "csv" {
		t.Fatalf("Format() = %q, want %q", got, "csv")
	}
}

func testFactoryWithConfig(t *testing.T, body string) *Factory {
	t.Helper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	return &Factory{
		ConfigPath: cfgPath,
		IOStreams: &IOStreams{
			In:     &bytes.Buffer{},
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
			IsTTY:  false,
		},
	}
}
