package completion_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmd"
)

// trackerServer serves /trackers.json with a single tracker named name and
// records whether it was asked.
func trackerServer(t *testing.T, name string) (*httptest.Server, *bool) {
	t.Helper()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/trackers.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"` + name + `"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &called
}

// isolateEnv points HOME at an empty directory and clears the REDMINE_*
// variables that select a server, so a test never reads the user's real
// ~/.redmine-cli.yaml. It returns the temporary home directory.
func isolateEnv(t *testing.T) string {
	t.Helper()
	for _, env := range []string{"REDMINE_SERVER", "REDMINE_API_KEY", "REDMINE_PROFILE", "REDMINE_STRICT"} {
		t.Setenv(env, "")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// complete runs `redmine __complete args...` and returns its output.
func complete(t *testing.T, args ...string) string {
	t.Helper()
	root := cmd.NewRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"__complete"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// Cobra parses the root command's persistent flags for `__complete` but does
// not run its PersistentPreRunE, so completion used to ignore --config,
// --profile and --server and always read ~/.redmine-cli.yaml.
func TestCompletion_HonorsGlobalFlags(t *testing.T) {
	home := isolateEnv(t)

	homeSrv, homeCalled := trackerServer(t, "FromHomeConfig")
	writeConfig(t, filepath.Join(home, ".redmine-cli.yaml"),
		"server: "+homeSrv.URL+"\napi_key: home\nauth_method: apikey\n")

	flagSrv, flagCalled := trackerServer(t, "FromFlagConfig")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, cfgPath, "server: "+flagSrv.URL+"\napi_key: flag\nauth_method: apikey\n")

	profileSrv, profileCalled := trackerServer(t, "FromProfile")
	profilesPath := filepath.Join(t.TempDir(), "profiles.yaml")
	writeConfig(t, profilesPath, "active_profile: default\nprofiles:\n"+
		"  default:\n    server: "+homeSrv.URL+"\n    api_key: home\n    auth_method: apikey\n"+
		"  work:\n    server: "+profileSrv.URL+"\n    api_key: work\n    auth_method: apikey\n")

	tests := []struct {
		name   string
		args   []string
		want   string
		called *bool
	}{
		{"config", []string{"--config", cfgPath, "issues", "list", "--tracker", ""}, "FromFlagConfig", flagCalled},
		{"config after the subcommand", []string{"issues", "list", "--config", cfgPath, "--tracker", ""}, "FromFlagConfig", flagCalled},
		{"profile", []string{"--config", profilesPath, "--profile", "work", "issues", "list", "--tracker", ""}, "FromProfile", profileCalled},
		{"server", []string{"--config", cfgPath, "--server", profileSrv.URL, "--api-key-file", writeKeyFile(t), "issues", "list", "--tracker", ""}, "FromProfile", profileCalled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*homeCalled, *flagCalled, *profileCalled = false, false, false
			out := complete(t, tt.args...)
			if !strings.Contains(out, tt.want) {
				t.Errorf("completions = %q, want %s", out, tt.want)
			}
			if !*tt.called {
				t.Error("the server selected by the flags was not asked")
			}
			if *homeCalled {
				t.Error("completion used ~/.redmine-cli.yaml despite the flags")
			}
		})
	}

	// Without flags, completion still uses the default config.
	*homeCalled = false
	if out := complete(t, "issues", "list", "--tracker", ""); !strings.Contains(out, "FromHomeConfig") || !*homeCalled {
		t.Errorf("completions = %q, want the default config's trackers", out)
	}
}

func writeKeyFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key")
	writeConfig(t, path, "flag-key\n")
	return path
}

// --api-key-file - would read the key from the terminal the user is typing
// in, so completion offers nothing instead of blocking on stdin.
func TestCompletion_APIKeyFileStdinSkipsServer(t *testing.T) {
	home := isolateEnv(t)
	srv, called := trackerServer(t, "FromServer")
	// Without the flags, the default config points at the server too.
	writeConfig(t, filepath.Join(home, ".redmine-cli.yaml"), "server: "+srv.URL+"\napi_key: home\nauth_method: apikey\n")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeConfig(t, cfgPath, "server: "+srv.URL+"\nauth_method: apikey\n")

	out := complete(t, "--config", cfgPath, "--api-key-file", "-", "issues", "list", "--tracker", "")
	if strings.Contains(out, "FromServer") || *called {
		t.Errorf("completion contacted the server with --api-key-file -: %q", out)
	}
}
