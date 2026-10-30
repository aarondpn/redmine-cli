package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

// TestGetIssue_ChildrenTreeAndNewFields pins that the recursive children tree
// and the fields added for Redmine 5.0+/6.0+/7.0 pass through get_issue and
// validate against the advertised output schema.
func TestGetIssue_ChildrenTreeAndNewFields(t *testing.T) {
	var gotInclude string
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue":{"id":7,"subject":"Parent",
			"status":{"id":5,"name":"Closed","is_closed":true},
			"closed_on":"2026-01-03T10:00:00Z","spent_hours":0.0,"total_spent_hours":1.5,"total_estimated_hours":2.0,
			"children":[{"id":8,"tracker":{"id":1,"name":"Bug"},"subject":"Child","children":[
				{"id":9,"tracker":{"id":1,"name":"Bug"},"subject":"Grandchild","children":[{"id":10,"subject":"Deep"}]}]}],
			"changesets":[{"revision":"abc","comments":"fix","committed_on":"2026-01-02T09:00:00Z"}],
			"allowed_statuses":[{"id":1,"name":"New","is_closed":false}],
			"reactions":[{"user":{"id":1,"name":"Admin"}}],
			"journals":[{"id":1,"user":{"id":1,"name":"Admin"},"notes":"n","created_on":"2026-01-02T00:00:00Z",
				"updated_on":"2026-01-02T01:00:00Z","updated_by":{"id":2,"name":"Other"},"reactions":[{"user":{"id":2,"name":"Other"}}]}]}}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_issue",
		Arguments: map[string]any{"id": 7, "includes": []string{"children", "changesets", "allowed_statuses", "reactions", "journals"}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if gotInclude != "children,changesets,allowed_statuses,reactions,journals" {
		t.Errorf("include = %q", gotInclude)
	}
	text := asText(t, res.Content[0])
	if !containsText(text,
		`"is_closed":true`, `"closed_on":"2026-01-03T10:00:00Z"`, `"spent_hours":0`, `"total_spent_hours":1.5`,
		`"total_estimated_hours":2`, `"subject":"Deep"`, `"revision":"abc"`, `"allowed_statuses":[`,
		`"reactions":[`, `"updated_on":"2026-01-02T01:00:00Z"`, `"updated_by":{`) {
		t.Errorf("missing new fields in output: %s", text)
	}
}

// TestOutputSchemaWithoutSecrets_IssueChildCycle guards the recursive
// models.IssueChild: the SDK's own inference rejects self-referential types,
// so the schema must be built here for every tool that returns an issue.
func TestOutputSchemaWithoutSecrets_IssueChildCycle(t *testing.T) {
	if outputSchemaWithoutSecrets[*models.Issue]() == nil {
		t.Fatal("get_issue output schema is nil; the SDK would panic inferring the recursive children type")
	}
	if outputSchemaWithoutSecrets[ops.IssuesListResult]() == nil {
		t.Fatal("list_issues output schema is nil")
	}
}
