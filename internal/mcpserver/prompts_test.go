package mcpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPromptDefinitions_Parse(t *testing.T) {
	defs, err := parsePromptDefinitions(fstest.MapFS{
		"triage_issue.md": {
			Data: []byte(`---
name: triage_issue
description: Example prompt
arguments:
  - name: issue_id
    required: true
---
Inspect issue #{{.issue_id}}.
`),
		},
	})
	if err != nil {
		t.Fatalf("parsePromptDefinitions: %v", err)
	}
	if _, ok := defs["triage_issue"]; !ok {
		t.Fatalf("triage_issue prompt missing: %+v", defs)
	}

	parsed, err := parsePromptDefinition("triage_issue.md", `---
name: triage_issue
description: Example prompt
arguments:
  - name: issue_id
    required: true
---
Inspect issue #{{.issue_id}}.
`)
	if err != nil {
		t.Fatalf("parsePromptDefinition: %v", err)
	}
	if parsed.Prompt.Name != "triage_issue" {
		t.Fatalf("prompt name = %q", parsed.Prompt.Name)
	}
	if len(parsed.Prompt.Arguments) != 1 || parsed.Prompt.Arguments[0].Name != "issue_id" || !parsed.Prompt.Arguments[0].Required {
		t.Fatalf("unexpected arguments: %+v", parsed.Prompt.Arguments)
	}
}

func TestPrompts_ListAndGet(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	ctx := context.Background()
	list, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}

	names := map[string]bool{}
	for _, prompt := range list.Prompts {
		names[prompt.Name] = true
	}
	for _, want := range []string{"triage_issue", "log_time_followup"} {
		if !names[want] {
			t.Fatalf("prompt %q missing from list", want)
		}
	}

	got, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "triage_issue",
		Arguments: map[string]string{"issue_id": "42", "project_hint": "demo"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(got.Messages))
	}
	text, ok := got.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", got.Messages[0].Content)
	}
	if !strings.Contains(text.Text, "issue #42") || !strings.Contains(text.Text, "project demo") {
		t.Fatalf("unexpected prompt text: %q", text.Text)
	}

	withoutOptional, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "triage_issue",
		Arguments: map[string]string{"issue_id": "99"},
	})
	if err != nil {
		t.Fatalf("GetPrompt without optional arg: %v", err)
	}
	optionalText, ok := withoutOptional.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", withoutOptional.Messages[0].Content)
	}
	if !strings.Contains(optionalText.Text, "issue #99") {
		t.Fatalf("expected issue id in prompt text: %q", optionalText.Text)
	}
	if strings.Contains(optionalText.Text, "in project") {
		t.Fatalf("optional clause should be omitted when project_hint is missing: %q", optionalText.Text)
	}

	if _, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "triage_issue",
		Arguments: map[string]string{},
	}); err == nil {
		t.Fatal("expected error when required argument is missing")
	}
}
