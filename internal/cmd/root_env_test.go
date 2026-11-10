package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// TestRootHonoursConfigProfileAndHeaderEnv runs the real root command, so
// the env vars travel the same path as for every command: PersistentPreRunE
// resolves REDMINE_CONFIG / REDMINE_PROFILE and Factory.Config merges
// REDMINE_HEADERS.
func TestRootHonoursConfigProfileAndHeaderEnv(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "env.yaml")
	content := `active_profile: a
profiles:
  a:
    server: https://a.example.com
    api_key: ka
  b:
    server: https://b.example.com
    api_key: kb
    headers:
      X-Profile: p
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"REDMINE_SERVER", "REDMINE_API_KEY", "REDMINE_CA_CERT"} {
		t.Setenv(v, "")
	}

	tests := []struct {
		name        string
		args        []string
		envProfile  string
		envHeaders  string
		wantProfile string
		wantServer  string
		wantHeaders string
		wantErr     string
	}{
		{name: "active profile from env config", wantProfile: "a", wantServer: "https://a.example.com"},
		{name: "env profile", envProfile: "b", wantProfile: "b", wantServer: "https://b.example.com", wantHeaders: "X-Profile"},
		{name: "flag profile wins", args: []string{"--profile", "a"}, envProfile: "b", wantProfile: "a", wantServer: "https://a.example.com"},
		{name: "env headers merged", envProfile: "b", envHeaders: "X-Env: e", wantProfile: "b", wantServer: "https://b.example.com", wantHeaders: "X-Env, X-Profile"},
		{name: "invalid env headers", envHeaders: "garbage", wantErr: config.HeadersEnv},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.ConfigEnv, cfgPath)
			t.Setenv(config.ProfileEnv, tt.envProfile)
			t.Setenv(config.HeadersEnv, tt.envHeaders)

			root, f := NewRootCmdWithFactory("test")
			out, errOut := &strings.Builder{}, &strings.Builder{}
			f.IOStreams.Out, f.IOStreams.ErrOut, f.IOStreams.IsTTY = out, errOut, false
			root.SetOut(out)
			root.SetErr(errOut)
			root.SetArgs(append(append([]string{}, tt.args...), "config", "-o", "json"))

			err := root.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Execute() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error = %v\nstderr: %s", err, errOut)
			}
			var got map[string]string
			if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
				t.Fatalf("output is not JSON: %v\n%s", err, out)
			}
			if got["profile"] != tt.wantProfile || got["server"] != tt.wantServer || got["headers"] != tt.wantHeaders {
				t.Fatalf("config = %v; want profile %q, server %q, headers %q", got, tt.wantProfile, tt.wantServer, tt.wantHeaders)
			}
		})
	}
}
