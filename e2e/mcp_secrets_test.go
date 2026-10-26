//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPStdio_NeverReturnsAPIKey reproduces the H-01 proof of concept
// against a live Redmine: /my/account.json returns the caller's api_key, and
// no MCP tool or resource may pass it on to the client.
func TestMCPStdio_NeverReturnsAPIKey(t *testing.T) {
	requireE2E(t)
	key := e2eAPIKey()
	if key == "" {
		t.Skip("REDMINE_E2E_API_KEY not set")
	}
	r := newCLIRunner(t, e2eBaseURL(), key)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx)

	var outputs []string
	for _, name := range []string{"get_my_account", "me", "list_users"} {
		out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name})
		if err != nil {
			t.Fatalf("CallTool %s: %v", name, err)
		}
		if out.IsError {
			t.Fatalf("%s returned IsError: %s", name, structuredJSON(out))
		}
		outputs = append(outputs, name+": "+structuredJSON(out))
	}
	for _, uri := range []string{"redmine://user/me"} {
		res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			t.Fatalf("ReadResource %s: %v", uri, err)
		}
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal %s: %v", uri, err)
		}
		outputs = append(outputs, uri+": "+string(b))
	}

	for _, out := range outputs {
		if strings.Contains(out, key) || strings.Contains(out, `"api_key"`) {
			t.Errorf("MCP output leaked the API key: %s", out)
		}
	}
}
