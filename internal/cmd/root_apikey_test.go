package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

func runRootForAPIKey(t *testing.T, stdin string, args ...string) (*cmdutil.Factory, error) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "profiles:\n  only:\n    server: https://only.example.com\n    auth_method: apikey\n    api_key: file-key\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	root, f := NewRootCmdWithFactory("test")
	f.IOStreams = &cmdutil.IOStreams{In: strings.NewReader(stdin), Out: &strings.Builder{}, ErrOut: &strings.Builder{}}
	root.SetOut(f.IOStreams.Out)
	root.SetErr(f.IOStreams.ErrOut)
	root.SetArgs(append([]string{"--config", cfgPath}, args...))
	return f, root.Execute()
}

func TestRootAPIKeyFileFlag(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyFile, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "file", args: []string{"--api-key-file", keyFile, "config"}, want: "from-file"},
		{name: "stdin", args: []string{"--api-key-file", "-", "config"}, stdin: "from-stdin\n", want: "from-stdin"},
		{name: "--api-key still works", args: []string{"--api-key", "from-flag", "config"}, want: "from-flag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := runRootForAPIKey(t, tc.stdin, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := f.Config()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.APIKey != tc.want {
				t.Fatalf("APIKey = %q, want %q", cfg.APIKey, tc.want)
			}
		})
	}
}

func TestRootAPIKeyAndFileAreMutuallyExclusive(t *testing.T) {
	_, err := runRootForAPIKey(t, "", "--api-key", "k", "--api-key-file", "-", "config")
	if err == nil || !strings.Contains(err.Error(), "api-key-file") {
		t.Fatalf("Execute error = %v, want mutually exclusive flag error", err)
	}
}
