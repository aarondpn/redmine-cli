//go:build unix

package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestScriptScaffoldWhoami runs the first command `extension create`
// suggests, through the real CLI and a nested `redmine api` call: it shows
// the user, and without a configured Redmine it fails with a readable
// message rather than a JSON error envelope.
func TestScriptScaffoldWhoami(t *testing.T) {
	work := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "extensions")
	run := func(config string, args ...string) (string, string, int) {
		t.Helper()
		return runInDir(t, work, extRoot, config, args...)
	}
	missing := filepath.Join(t.TempDir(), "none.yaml")
	for _, args := range [][]string{{"extension", "create", "hello"}, {"extension", "install", "./redmine-cli-hello"}} {
		if _, stderr, code := run(missing, args...); code != 0 {
			t.Fatalf("%v: exit %d\n%s", args, code, stderr)
		}
	}

	stdout, stderr, code := run(missing, "hello", "whoami")
	if code == 0 || strings.Contains(stderr, `{"error"`) || !strings.Contains(stderr, "Error: server URL not configured") {
		t.Errorf("unconfigured whoami: exit %d, stdout %q, stderr %q; want a readable error", code, stdout, stderr)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/current.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":7,"login":"jdoe","firstname":"Jane","lastname":"Doe"}}`))
	}))
	defer srv.Close()
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("server: "+srv.URL+"\napi_key: test\nauth_method: apikey\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = run(cfg, "hello", "whoami")
	if code != 0 {
		t.Fatalf("whoami: exit %d\n%s", code, stderr)
	}
	for _, want := range []string{"7", "jdoe", "Jane Doe"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("whoami output lacks %q:\n%s", want, stdout)
		}
	}
}

// runInDir runs the CLI in dir with the extensions directory extRoot and the
// config file config, and returns stdout, stderr and the exit code.
func runInDir(t *testing.T, dir, extRoot, config string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir = dir
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "REDMINE_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "REDMINE_MAIN_HELPER=1", "REDMINE_NO_UPDATE_CHECK=1",
		"REDMINE_EXTENSIONS_DIR="+extRoot, "REDMINE_CONFIG="+config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	code := 0
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), code
}

// TestUnknownManifestFieldWarnsAtRuntime: a manifest field this
// redmine-cli does not know (one added by a newer version) does not stop the
// extension from running; it is reported on stderr once per invocation, not
// during shell completion, while `extension check` still rejects it.
func TestUnknownManifestFieldWarnsAtRuntime(t *testing.T) {
	extRoot := filepath.Join(t.TempDir(), "extensions")
	dir := filepath.Join(extRoot, "redmine-cli-dev")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redmine-cli-dev"), []byte("#!/bin/sh\necho '{\"ok\":true}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "schema_version: 1\nname: dev\ndescription: Test\ncommands:\n  - name: list\n    description: List things\n    future_field: true\n"
	if err := os.WriteFile(filepath.Join(dir, "redmine-extension.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "none.yaml")
	const warning = "Warning: extension dev: redmine-extension.yml:7: "

	stdout, stderr, code := runInDir(t, t.TempDir(), extRoot, config, "dev", "list", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"ok": true`) {
		t.Fatalf("dev list: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if n := strings.Count(stderr, warning); n != 1 || !strings.Contains(stderr, "future_field") {
		t.Errorf("stderr has the warning %d times, want once:\n%s", n, stderr)
	}
	if strings.Contains(stdout, "Warning") {
		t.Errorf("the warning reached stdout: %q", stdout)
	}

	// mcp tools lists the extensions a second time in the invocation.
	stdout, stderr, code = runInDir(t, t.TempDir(), extRoot, config, "mcp", "tools", "-o", "json")
	if code != 0 || !strings.Contains(stdout, "dev_list") {
		t.Fatalf("mcp tools: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if n := strings.Count(stderr, warning); n != 1 {
		t.Errorf("mcp tools: stderr has the warning %d times, want once:\n%s", n, stderr)
	}

	stdout, stderr, _ = runInDir(t, t.TempDir(), extRoot, config, "__complete", "dev", "")
	if strings.Contains(stdout+stderr, "Warning") {
		t.Errorf("completion printed the warning: stdout %q, stderr %q", stdout, stderr)
	}
	if !strings.Contains(stdout, "list") {
		t.Errorf("completion lacks the command: %q", stdout)
	}

	_, stderr, code = runInDir(t, t.TempDir(), extRoot, config, "extension", "check", "dev")
	if code == 0 || !strings.Contains(stderr, "future_field") {
		t.Errorf("extension check: exit %d, stderr %q; want the unknown field rejected", code, stderr)
	}
}
