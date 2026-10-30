//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// These tests cover --custom-field (and the MCP custom_field_values) on every
// resource whose safe_attributes accept custom fields. Values are verified
// through rawGET, which bypasses the code under test.

// seededCustomFieldID returns the ID of the seeded custom field with the given
// name and customized type.
func seededCustomFieldID(t *testing.T, r *cliRunner, name, customizedType string) int {
	t.Helper()
	var fields []struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		CustomizedType string `json:"customized_type"`
	}
	r.runJSON(t, &fields, "custom-fields", "list")
	for _, f := range fields {
		if f.Name == name && f.CustomizedType == customizedType {
			return f.ID
		}
	}
	t.Fatalf("seeded %s custom field %q not found; recreate the stack so bootstrap-redmine.sh seeds it (got %+v)", customizedType, name, fields)
	return 0
}

// rawCustomFieldValue returns the "value" of custom field fieldID in the
// resource at path, whose payload sits under root.
func rawCustomFieldValue(t *testing.T, path, root string, fieldID int) any {
	t.Helper()
	cf := firstWhere(field(rawGET(t, path), root, "custom_fields"), func(m map[string]any) bool {
		return jsonInt(m["id"]) == fieldID
	})
	if cf == nil {
		t.Fatalf("%s has no custom field %d", path, fieldID)
	}
	return cf["value"]
}

// stringSet normalizes a string or array value to a sorted []string.
func stringSet(v any) []string {
	out := []string{}
	switch x := v.(type) {
	case string:
		if x != "" {
			out = append(out, x)
		}
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	}
	slices.Sort(out)
	return out
}

func requireValueSet(t *testing.T, what string, got any, want ...string) {
	t.Helper()
	slices.Sort(want)
	if g := stringSet(got); !reflect.DeepEqual(g, want) {
		t.Fatalf("%s = %v (raw %#v), want %v", what, g, got, want)
	}
}

func requireValue(t *testing.T, what string, got any, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %#v, want %q", what, got, want)
	}
}

func TestCustomFieldInput_IssueMultiValue(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	tagsID := seededCustomFieldID(t, r, "E2E Tags", "issue")

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E multi-value custom field",
		"--custom-field", "E2E Tags=Alpha",
		"--custom-field", "E2E Tags=Beta")
	path := "/issues/" + strconv.Itoa(created.ID) + ".json"
	requireValueSet(t, "tags after create", rawCustomFieldValue(t, path, "issue", tagsID), "Alpha", "Beta")

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "update", strconv.Itoa(created.ID), "--custom-field", "E2E Tags=Gamma")
	requireValueSet(t, "tags after update", rawCustomFieldValue(t, path, "issue", tagsID), "Gamma")

	r.runJSON(t, &env, "issues", "update", strconv.Itoa(created.ID), "--custom-field", "E2E Tags=")
	requireValueSet(t, "tags after clear", rawCustomFieldValue(t, path, "issue", tagsID))
}

func TestCustomFieldInput_IssueNameScopedToType(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issueFieldID := seededCustomFieldID(t, r, "E2E Shared Code", "issue")

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E type-scoped custom field name",
		"--custom-field", "E2E Shared Code=ISSUE-1")
	requireValue(t, "issue E2E Shared Code",
		rawCustomFieldValue(t, "/issues/"+strconv.Itoa(created.ID)+".json", "issue", issueFieldID), "ISSUE-1")
}

func TestCustomFieldInput_TimeEntry(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	entryTagsID := seededCustomFieldID(t, r, "E2E Entry Tags", "time_entry")
	sharedID := seededCustomFieldID(t, r, "E2E Shared Code", "time_entry")

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "time", "log",
		"--issue", strconv.Itoa(issue.ID),
		"--hours", "1",
		"--activity", firstActivityName(t, r),
		"--custom-field", "E2E Entry Tags=Billable",
		"--custom-field", "E2E Entry Tags=Onsite",
		"--custom-field", "E2E Shared Code=TE-1")
	path := "/time_entries/" + strconv.Itoa(created.ID) + ".json"
	requireValueSet(t, "entry tags", rawCustomFieldValue(t, path, "time_entry", entryTagsID), "Billable", "Onsite")
	requireValue(t, "time entry E2E Shared Code", rawCustomFieldValue(t, path, "time_entry", sharedID), "TE-1")

	var env actionEnvelope
	r.runJSON(t, &env, "time", "update", strconv.Itoa(created.ID),
		"--custom-field", strconv.Itoa(entryTagsID)+"=Overtime")
	requireValueSet(t, "entry tags after update", rawCustomFieldValue(t, path, "time_entry", entryTagsID), "Overtime")
}

func TestCustomFieldInput_TimeEntryRejectsOtherTypeName(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	_, stderr := r.runExpectError(t, "time", "log",
		"--issue", strconv.Itoa(issue.ID),
		"--hours", "1",
		"--activity", firstActivityName(t, r),
		"--custom-field", "E2E Severity=High")
	if !strings.Contains(string(stderr), "custom field of type issue") {
		t.Fatalf("stderr does not explain the type mismatch:\n%s", stderr)
	}

	var entries []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &entries, "time", "list", "--issue", strconv.Itoa(issue.ID))
	if len(entries) != 0 {
		t.Fatalf("a time entry was created despite the rejected custom field: %+v", entries)
	}
}

func TestCustomFieldInput_Version(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	ownerID := seededCustomFieldID(t, r, "E2E Version Owner", "version")

	var created struct {
		ID int `json:"id"`
	}
	// The value contains spaces and an "=" (#155): it is everything after the
	// first "=".
	r.runJSON(t, &created, "versions", "create",
		"--project", proj.Identifier,
		"--name", "v-"+uniqueShortSuffix(t),
		"--custom-field", "E2E Version Owner=Team = A")
	path := "/versions/" + strconv.Itoa(created.ID) + ".json"
	requireValue(t, "version owner", rawCustomFieldValue(t, path, "version", ownerID), "Team = A")

	var env actionEnvelope
	r.runJSON(t, &env, "versions", "update", strconv.Itoa(created.ID), "--custom-field", "E2E Version Owner=Team B")
	requireValue(t, "version owner after update", rawCustomFieldValue(t, path, "version", ownerID), "Team B")
}

func TestCustomFieldInput_Project(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	labelsID := seededCustomFieldID(t, r, "E2E Project Labels", "project")

	var env actionEnvelope
	r.runJSON(t, &env, "projects", "update", proj.Identifier,
		"--custom-field", "E2E Project Labels=red",
		"--custom-field", "E2E Project Labels=blue")
	requireValueSet(t, "project labels",
		rawCustomFieldValue(t, "/projects/"+proj.Identifier+".json", "project", labelsID), "red", "blue")
}

func TestCustomFieldInput_User(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	skillsID := seededCustomFieldID(t, r, "E2E User Skills", "user")

	user := createTestUser(t, r,
		"--custom-field", "E2E User Skills=Go",
		"--custom-field", "E2E User Skills=SQL")
	path := "/users/" + strconv.Itoa(user.ID) + ".json"
	requireValueSet(t, "user skills", rawCustomFieldValue(t, path, "user", skillsID), "Go", "SQL")

	var env actionEnvelope
	r.runJSON(t, &env, "users", "update", strconv.Itoa(user.ID), "--custom-field", "E2E User Skills=Ruby")
	requireValueSet(t, "user skills after update", rawCustomFieldValue(t, path, "user", skillsID), "Ruby")
}

func TestCustomFieldInput_MyAccount(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	skillsID := seededCustomFieldID(t, r, "E2E User Skills", "user")

	// The admin account is shared across the suite.
	t.Cleanup(func() {
		var env actionEnvelope
		r.runJSON(t, &env, "my-account", "update", "--custom-field", "E2E User Skills=")
	})

	var env actionEnvelope
	r.runJSON(t, &env, "my-account", "update",
		"--custom-field", "E2E User Skills=Go",
		"--custom-field", "E2E User Skills=Ruby")
	requireValueSet(t, "my account skills", rawCustomFieldValue(t, "/my/account.json", "user", skillsID), "Go", "Ruby")
}

func TestCustomFieldInput_Group(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	codeID := seededCustomFieldID(t, r, "E2E Group Code", "group")

	grp := createTestGroup(t, r, "--custom-field", "E2E Group Code=G-1")
	path := "/groups/" + strconv.Itoa(grp.ID) + ".json"
	requireValue(t, "group code", rawCustomFieldValue(t, path, "group", codeID), "G-1")

	var env actionEnvelope
	r.runJSON(t, &env, "groups", "update", strconv.Itoa(grp.ID), "--custom-field", "E2E Group Code=G-2")
	requireValue(t, "group code after update", rawCustomFieldValue(t, path, "group", codeID), "G-2")
}

func TestCustomFieldInput_MultipleValuesOnSingleField(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	notesID := seededCustomFieldID(t, r, "E2E Notes", "issue")
	tracker := firstTrackerName(t, r)

	// By name the CLI knows the field is single-value and refuses locally.
	_, stderr := r.runExpectError(t, "issues", "create",
		"--project", proj.Identifier, "--tracker", tracker, "--subject", "E2E single-value field",
		"--custom-field", "E2E Notes=a", "--custom-field", "E2E Notes=b")
	if !strings.Contains(string(stderr), "does not accept multiple values") {
		t.Fatalf("stderr does not explain the rejection:\n%s", stderr)
	}

	// By ID there is no definition to check; Redmine rejects it and the CLI
	// adds a hint.
	stderr = r.runExpectJSONError(t, "issues", "create",
		"--project", proj.Identifier, "--tracker", tracker, "--subject", "E2E single-value field",
		"--custom-field", strconv.Itoa(notesID)+"=a", "--custom-field", strconv.Itoa(notesID)+"=b")
	var env struct {
		Error struct {
			Code string `json:"code"`
			Hint string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(lastLine(stderr), &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstderr:\n%s", err, stderr)
	}
	if env.Error.Code != "validation_failed" || !strings.Contains(env.Error.Hint, "multi-value") {
		t.Fatalf("unexpected error envelope: %+v\nstderr:\n%s", env.Error, stderr)
	}
}

func TestMCPStdio_CustomFieldValues(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	tagsID := seededCustomFieldID(t, r, "E2E Tags", "issue")
	notesID := seededCustomFieldID(t, r, "E2E Notes", "issue")
	entryTagsID := seededCustomFieldID(t, r, "E2E Entry Tags", "time_entry")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx, "--enable-writes")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, name := range []string{"create_issue", "create_time_entry", "update_version", "update_user", "update_my_account", "create_group"} {
		var found bool
		for _, tool := range tools.Tools {
			if tool.Name != name {
				continue
			}
			found = true
			schema, _ := json.Marshal(tool.InputSchema)
			if !strings.Contains(string(schema), `"custom_field_values"`) || !strings.Contains(string(schema), `"anyOf"`) {
				t.Errorf("%s input schema lacks a string-or-array custom_field_values: %s", name, schema)
			}
		}
		if !found {
			t.Errorf("tool %s not listed", name)
		}
	}

	out := mcpCallOK(t, ctx, session, "create_issue", map[string]any{
		"project_id": proj.Identifier,
		"subject":    "E2E MCP custom field values",
		"custom_field_values": map[string]any{
			strconv.Itoa(tagsID):  []string{"Alpha", "Gamma"},
			strconv.Itoa(notesID): "two words",
		},
	})
	id := mcpExtIssueIDFromResult(t, out)
	if id == 0 {
		t.Fatalf("create_issue returned no ID: %s", structuredJSON(out))
	}
	path := "/issues/" + strconv.Itoa(id) + ".json"
	requireValueSet(t, "mcp tags", rawCustomFieldValue(t, path, "issue", tagsID), "Alpha", "Gamma")
	requireValue(t, "mcp notes", rawCustomFieldValue(t, path, "issue", notesID), "two words")

	// A one-element array is sent as a plain string.
	out = mcpCallOK(t, ctx, session, "create_time_entry", map[string]any{
		"issue_id":            issue.ID,
		"hours":               1,
		"custom_field_values": map[string]any{strconv.Itoa(entryTagsID): []string{"Overtime"}},
	})
	teID := mcpExtIssueIDFromResult(t, out)
	if teID == 0 {
		t.Fatalf("create_time_entry returned no ID: %s", structuredJSON(out))
	}
	requireValueSet(t, "mcp entry tags",
		rawCustomFieldValue(t, "/time_entries/"+strconv.Itoa(teID)+".json", "time_entry", entryTagsID), "Overtime")

	// Names are not accepted; the schema only allows numeric ID keys. The SDK
	// reports that either as a protocol error or as a tool error.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_issue",
		Arguments: map[string]any{
			"project_id":          proj.Identifier,
			"subject":             "E2E MCP name key",
			"custom_field_values": map[string]any{"E2E Tags": "Alpha"},
		},
	})
	if err == nil && !res.IsError {
		t.Fatalf("create_issue with a name key succeeded: %s", structuredJSON(res))
	}
}
