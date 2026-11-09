package mcpserver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

func TestRedactSecretText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"json", `{"user":{"login":"a","api_key":"SECRETKEY"}}`, `{"user":{"login":"a","api_key":"[REDACTED]"}}`},
		{"json with spaces and escapes", `{"api_key" :  "SEC\"RET"}`, `{"api_key" :  "[REDACTED]"}`},
		{"json quoted in a string", `{"error":"{\"api_key\":\"SECRETKEY\"}"}`, `{"error":"{\"api_key\":\"[REDACTED]\"}"}`},
		{"non-string json value", `{"api_key": 12345}`, `{"api_key": [REDACTED]}`},
		{"yaml", "login: a\napi_key: SECRETKEY\n", "login: a\napi_key: [REDACTED]\n"},
		{"env", "REDMINE_API_KEY=SECRETKEY other", "REDMINE_API_KEY=[REDACTED] other"},
		{"detail output", "Login:    a\nAPI Key:  SECRETKEY\n", "Login:    a\nAPI Key:  [REDACTED]\n"},
		{"colored detail output", "\x1b[1mAPI Key:\x1b[0m  SECRETKEY\n", "\x1b[1mAPI Key:\x1b[0m  [REDACTED]\n"},
		{"several", `{"api_key":"A"} {"api_key":"B"}`, `{"api_key":"[REDACTED]"} {"api_key":"[REDACTED]"}`},
		{"other keys kept", `{"api_keys":"x","monkey":"y"} key: z`, `{"api_keys":"x","monkey":"y"} key: z`},
		{"prose kept", "the api key was rotated", "the api key was rotated"},
		{"nothing to do", "plain text", "plain text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactSecretText(tt.in)
			if got != tt.want {
				t.Errorf("redactSecretText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtensionToolResult_RedactsEveryShape(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   string
	}{
		{
			name:   "single document",
			stdout: `{"user":{"login":"a","api_key":"SECRETKEY"}}`,
			want:   `{"user":{"login":"a"}}`,
		},
		{
			name:   "two documents in a row",
			stdout: "{\"user\":{\"login\":\"a\",\"api_key\":\"SECRETKEY\"}}\n{\"done\":true}\n",
			want:   "{\"user\":{\"login\":\"a\"}}\n{\"done\":true}\n",
		},
		{
			name:   "ndjson keeps large numbers",
			stdout: "{\"id\":12345678901234567890,\"api_key\":\"SECRETKEY\"}\n[{\"api_key\":\"X\"}]\n",
			want:   "{\"id\":12345678901234567890}\n[{}]\n",
		},
		{
			name:   "json followed by text",
			stdout: "{\"user\":{\"api_key\":\"SECRETKEY\"}}\ndone\n",
			want:   "{\"user\":{\"api_key\":\"[REDACTED]\"}}\ndone\n",
		},
		{
			name:   "plain text quoting a response",
			stdout: "got user a (api_key: SECRETKEY)\n",
			want:   "got user a (api_key: [REDACTED])\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := extensionToolResult([]byte(tt.stdout))
			if res.IsError {
				t.Fatalf("unexpected error result: %+v", res.Content)
			}
			got := asText(t, res.Content[0])
			if got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "SECRETKEY") {
				t.Errorf("the API key leaked: %q", got)
			}
		})
	}
}

// TestExtensionTools_RedactsErrorMessages runs a tool whose extension fails
// with a Redmine response on stderr: the isError text must not carry the
// API key either.
func TestExtensionTools_RedactsErrorMessages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the extension is a sh script")
	}
	unsetEnv(t, protocolEnv...)
	path := filepath.Join(t.TempDir(), "redmine-cli-leak")
	body := "#!/bin/sh\ncat >/dev/null\nif [ \"$REDMINE_EXT_COMMAND\" = fail ]; then\n" +
		"  echo '{\"user\":{\"api_key\":\"SECRETKEY\"}} rejected' >&2\n  exit 1\nfi\n" +
		"printf '{\"user\":{\"login\":\"a\",\"api_key\":\"SECRETKEY\"}}\\n{\"done\":true}\\n'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	tools := []ExtensionTool{
		{Extension: "leak", Path: path, Command: &spec.Command{Name: "fail", Description: "Fails"}, Env: runner.Env{}},
		{Extension: "leak", Path: path, Command: &spec.Command{Name: "multi", Description: "Two documents"}, Env: runner.Env{}},
	}
	cs := newExtensionSession(t, Options{ExtensionTools: tools})

	res := callTool(t, cs, "leak_fail", map[string]any{})
	text := resultText(t, res)
	if !res.IsError || strings.Contains(text, "SECRETKEY") || !strings.Contains(text, `"api_key":"[REDACTED]"`) {
		t.Fatalf("isError=%v text=%q", res.IsError, text)
	}

	res = callTool(t, cs, "leak_multi", map[string]any{})
	text = resultText(t, res)
	if res.IsError || strings.Contains(text, "SECRETKEY") || !strings.Contains(text, `{"done":true}`) {
		t.Fatalf("isError=%v text=%q", res.IsError, text)
	}
}
