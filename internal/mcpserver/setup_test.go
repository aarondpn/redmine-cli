package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectSetupServer(t *testing.T, problem string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	srv := BuildSetupServer(problem, Options{Version: "v0"})
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		_ = ss.Close()
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Close()
	})
	return cs
}

func TestSetupServer_ListsOnlySetupStatus(t *testing.T) {
	cs := connectSetupServer(t, "server URL not configured")
	ctx := context.Background()

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 1 || res.Tools[0].Name != SetupToolName {
		t.Fatalf("tools = %+v, want only %s", res.Tools, SetupToolName)
	}
	if ro := res.Tools[0].Annotations; ro == nil || !ro.ReadOnlyHint {
		t.Error("setup_status must be annotated read-only")
	}

	// No tool may take input: nothing here can carry a key, password or token.
	schema, err := json.Marshal(res.Tools[0].InputSchema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var in struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(schema, &in); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if len(in.Properties) != 0 {
		t.Errorf("setup_status accepts input %v, want none", in.Properties)
	}

	resources, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(resources.Resources) != 0 {
		t.Errorf("resources = %+v, want none", resources.Resources)
	}
}

func TestSetupServer_InstructionsAndToolResult(t *testing.T) {
	const problem = "keyring secret missing for profile work"
	cs := connectSetupServer(t, problem)

	want := []string{
		"redmine auth login",
		"--api-key-file -",
		"REDMINE_SERVER",
		"REDMINE_API_KEY",
		"restart",
		problem,
	}
	instructions := cs.InitializeResult().Instructions
	for _, w := range want {
		if !strings.Contains(instructions, w) {
			t.Errorf("server instructions missing %q:\n%s", w, instructions)
		}
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: SetupToolName})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("setup_status returned an error: %+v", res.Content)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	var status SetupStatus
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("unmarshal %q: %v", text, err)
	}
	if status.Configured || status.Problem != problem {
		t.Errorf("status = %+v", status)
	}
	for _, w := range want {
		if !strings.Contains(status.Instructions, w) {
			t.Errorf("tool instructions missing %q", w)
		}
	}
}
