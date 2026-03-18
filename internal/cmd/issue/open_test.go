package issue

import (
	"bytes"
	"os"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
)

func newTestFactory(cfg *config.Config) *cmdutil.Factory {
	f := &cmdutil.Factory{
		IOStreams: &cmdutil.IOStreams{
			In:     os.Stdin,
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
		},
	}
	// Pre-load config by calling SetConfig helper or using a config file.
	// We use a temp config file approach.
	return f
}

func TestOpenURL(t *testing.T) {
	// Stub openBrowser to capture the URL instead of launching a browser.
	var openedURL string
	origOpen := openBrowser
	t.Cleanup(func() { openBrowser = origOpen })
	openBrowser = func(url string) error {
		openedURL = url
		return nil
	}

	// Write a temporary config file.
	tmp := t.TempDir()
	cfgPath := tmp + "/config.yaml"
	if err := os.WriteFile(cfgPath, []byte("server: https://redmine.example.com\napi_key: test\nauth_method: apikey\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     os.Stdin,
			Out:    out,
			ErrOut: &bytes.Buffer{},
		},
	}

	cmd := NewCmdOpen(f)
	cmd.SetArgs([]string{"123"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://redmine.example.com/issues/123"
	if openedURL != expected {
		t.Errorf("expected URL %q, got %q", expected, openedURL)
	}
	if !bytes.Contains(out.Bytes(), []byte(expected)) {
		t.Errorf("expected output to contain %q, got %q", expected, out.String())
	}
}

func TestOpenTrailingSlash(t *testing.T) {
	var openedURL string
	origOpen := openBrowser
	t.Cleanup(func() { openBrowser = origOpen })
	openBrowser = func(url string) error {
		openedURL = url
		return nil
	}

	tmp := t.TempDir()
	cfgPath := tmp + "/config.yaml"
	if err := os.WriteFile(cfgPath, []byte("server: https://redmine.example.com/\napi_key: test\nauth_method: apikey\n"), 0644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     os.Stdin,
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
		},
	}

	cmd := NewCmdOpen(f)
	cmd.SetArgs([]string{"456"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://redmine.example.com/issues/456"
	if openedURL != expected {
		t.Errorf("expected URL %q, got %q", expected, openedURL)
	}
}

func TestOpenInvalidID(t *testing.T) {
	f := &cmdutil.Factory{
		IOStreams: &cmdutil.IOStreams{
			In:     os.Stdin,
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
		},
	}

	cmd := NewCmdOpen(f)
	cmd.SetArgs([]string{"abc"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-numeric ID")
	}
}
