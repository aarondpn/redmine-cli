// Package testutil provides shared test helpers for command-layer tests.
package testutil

import (
	"bytes"
	"os"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

// NewFactory creates a Factory pointing at the given test server with minimal config.
func NewFactory(t *testing.T, serverURL string) *cmdutil.Factory {
	t.Helper()
	return NewFactoryWithConfig(t, serverURL, "")
}

// NewFactoryWithConfig creates a Factory with additional YAML config lines appended.
func NewFactoryWithConfig(t *testing.T, serverURL string, extraYAML string) *cmdutil.Factory {
	t.Helper()

	cfgPath := t.TempDir() + "/config.yaml"
	cfg := "server: " + serverURL + "\napi_key: test\nauth_method: apikey\n" + extraYAML
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	return &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     &bytes.Buffer{},
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
			IsTTY:  false,
		},
	}
}

// Stdout returns the contents of the factory's stdout buffer.
func Stdout(f *cmdutil.Factory) string {
	return f.IOStreams.Out.(*bytes.Buffer).String()
}

// Stderr returns the contents of the factory's stderr buffer.
func Stderr(f *cmdutil.Factory) string {
	return f.IOStreams.ErrOut.(*bytes.Buffer).String()
}
