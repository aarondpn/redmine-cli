package issue

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

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
	errOut := f.IOStreams.ErrOut.(*bytes.Buffer)
	if !bytes.Contains(errOut.Bytes(), []byte(expected)) {
		t.Errorf("expected stderr to contain %q, got %q", expected, errOut.String())
	}
}

func TestOpenURL_JSONEnvelope(t *testing.T) {
	var openedURL string
	origOpen := openBrowser
	t.Cleanup(func() { openBrowser = origOpen })
	openBrowser = func(url string) error {
		openedURL = url
		return nil
	}

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
	cmd.SetArgs([]string{"123", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if openedURL != "https://redmine.example.com/issues/123" {
		t.Fatalf("unexpected URL %q", openedURL)
	}

	var env struct {
		Ok       bool   `json:"ok"`
		Action   string `json:"action"`
		Resource string `json:"resource"`
		ID       int    `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("expected JSON output, got: %v\n%s", err, out.String())
	}
	if !env.Ok || env.Action != "opened" || env.Resource != "issue" || env.ID != 123 {
		t.Fatalf("unexpected envelope: %+v", env)
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
