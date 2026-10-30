package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

// customFieldInputTypes lists the ops inputs whose custom_field_values must
// advertise the string-or-array schema.
var customFieldInputTypes = map[string]any{
	"CreateIssueInput":   inputSchemaFor[ops.CreateIssueInput](),
	"UpdateIssueInput":   inputSchemaFor[ops.UpdateIssueInput](),
	"CreateProjectInput": inputSchemaFor[ops.CreateProjectInput](),
	"UpdateProjectInput": inputSchemaFor[ops.UpdateProjectInput](),
}

func TestInputSchemaFor_CustomFieldValues(t *testing.T) {
	for name, raw := range customFieldInputTypes {
		t.Run(name, func(t *testing.T) {
			schema, ok := raw.(*jsonschema.Schema)
			if !ok || schema == nil {
				t.Fatalf("schema = %T, want *jsonschema.Schema", raw)
			}
			if schema.Type != "object" {
				t.Fatalf("type = %q", schema.Type)
			}
			prop := schema.Properties["custom_field_values"]
			if prop == nil {
				t.Fatal("no custom_field_values property")
			}
			if prop.Description == "" {
				t.Error("description is empty")
			}
			if prop.PropertyNames == nil || prop.PropertyNames.Pattern != "^[1-9][0-9]*$" {
				t.Errorf("propertyNames = %+v", prop.PropertyNames)
			}
			ap := prop.AdditionalProperties
			if ap == nil || len(ap.AnyOf) != 2 {
				t.Fatalf("additionalProperties = %+v", ap)
			}
			if ap.AnyOf[0].Type != "string" {
				t.Errorf("anyOf[0] = %+v", ap.AnyOf[0])
			}
			if ap.AnyOf[1].Type != "array" || ap.AnyOf[1].Items == nil || ap.AnyOf[1].Items.Type != "string" {
				t.Errorf("anyOf[1] = %+v", ap.AnyOf[1])
			}
		})
	}
}

func TestInputSchemaFor_UnchangedForPlainInputs(t *testing.T) {
	got, err := json.Marshal(inputSchemaFor[ops.ListIssuesInput]())
	if err != nil {
		t.Fatal(err)
	}
	base, err := jsonschema.ForType(reflect.TypeFor[ops.ListIssuesInput](), &jsonschema.ForOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(base)
	if string(got) != string(want) {
		t.Errorf("schema changed for an input without custom fields:\n got %s\nwant %s", got, want)
	}
}

// captureBodies records the JSON body of every request that reaches the API
// and answers with a minimal resource.
type captureBodies struct {
	mu     sync.Mutex
	bodies []map[string]any
	hits   atomic.Int32
	reply  string
}

func (h *captureBodies) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.hits.Add(1)
	raw, _ := io.ReadAll(r.Body)
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	h.mu.Lock()
	h.bodies = append(h.bodies, parsed)
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if h.reply == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_, _ = w.Write([]byte(h.reply))
}

func (h *captureBodies) last(t *testing.T, key string) map[string]any {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.bodies) == 0 {
		t.Fatal("no request captured")
	}
	obj, ok := h.bodies[len(h.bodies)-1][key].(map[string]any)
	if !ok {
		t.Fatalf("body lacks %q: %v", key, h.bodies[len(h.bodies)-1])
	}
	return obj
}

func TestCreateIssueTool_CustomFieldValuesStringOrArray(t *testing.T) {
	h := &captureBodies{reply: `{"issue":{"id":1,"subject":"s"}}`}
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_issue",
		Arguments: map[string]any{
			"project_id": 1,
			"subject":    "s",
			"custom_field_values": map[string]any{
				"5": "a",
				"6": []string{"x", "y"},
				"7": []string{},
				"8": []string{"only"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	got := h.last(t, "issue")["custom_field_values"]
	want := map[string]any{"5": "a", "6": []any{"x", "y"}, "7": "", "8": "only"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}
}

func TestCreateIssueTool_CustomFieldValuesRejectsBadInput(t *testing.T) {
	tests := map[string]map[string]any{
		"number":        {"5": 3},
		"name key":      {"Severity": "x"},
		"number in arr": {"5": []int{1}},
	}
	for name, cfv := range tests {
		t.Run(name, func(t *testing.T) {
			h := &captureBodies{reply: `{"issue":{"id":1,"subject":"s"}}`}
			apiClient, closeTS := newTestAPIClient(t, h)
			defer closeTS()
			cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
			defer cleanup()

			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "create_issue",
				Arguments: map[string]any{"project_id": 1, "subject": "s", "custom_field_values": cfv},
			})
			if err == nil && (res == nil || !res.IsError) {
				t.Fatalf("expected an error result, got %+v", res)
			}
			if h.hits.Load() != 0 {
				t.Errorf("API was hit %d times", h.hits.Load())
			}
		})
	}
}

func TestUpdateProjectTool_CustomFieldValuesMulti(t *testing.T) {
	h := &captureBodies{}
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "update_project",
		Arguments: map[string]any{
			"identifier":          "demo",
			"custom_field_values": map[string]any{"9": []string{"a", "b"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	got := h.last(t, "project")["custom_field_values"]
	if want := map[string]any{"9": []any{"a", "b"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}
}

func TestToolsList_CustomFieldValuesSchemaAdvertised(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.NotFoundHandler())
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "create_issue" {
			continue
		}
		data, _ := json.Marshal(tool.InputSchema)
		for _, want := range []string{`"anyOf"`, `"propertyNames"`} {
			if !strings.Contains(string(data), want) {
				t.Errorf("create_issue input schema lacks %s: %s", want, data)
			}
		}
		return
	}
	t.Fatal("create_issue not listed")
}
