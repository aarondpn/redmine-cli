//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil/contract"
)

// The contract tests compare what Redmine returned with what the CLI and the
// MCP server produced. This file holds the plumbing: fetching the raw
// payload, discovering the seeded world, and running the CLI / an MCP tool
// into the same untyped JSON tree the diff engine consumes.

const contractProject = "e2e-contract"

// rawGET fetches path from Redmine with the API key, bypassing the code under
// test (the CLI's own `api` command would route through the same HTTP and
// JSON handling that the contract is meant to verify).
func rawGET(t *testing.T, path string) any {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(e2eBaseURL(), "/")+path, nil)
	if err != nil {
		t.Fatalf("build request for %s: %v", path, err)
	}
	req.Header.Set("X-Redmine-API-Key", e2eAPIKey())
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: HTTP %d\n%s", path, resp.StatusCode, body)
	}
	v, err := contract.Decode(body)
	if err != nil {
		t.Fatalf("decode %s: %v\n%s", path, err, body)
	}
	return v
}

// runJSONAny runs the CLI and returns its JSON output as an untyped tree.
// It deliberately avoids the typed models: they are what is being tested.
func (r *cliRunner) runJSONAny(t *testing.T, args ...string) any {
	t.Helper()
	out := r.run(t, args...)
	v, err := contract.Decode(out)
	if err != nil {
		t.Fatalf("decode JSON for %q: %v\nstdout:\n%s", strings.Join(args, " "), err, out)
	}
	return v
}

// mcpCallAny calls an MCP tool and returns the structured result as an
// untyped tree.
func mcpCallAny(t *testing.T, ctx context.Context, session *mcp.ClientSession, tool string, args map[string]any) any {
	t.Helper()
	out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", tool, err)
	}
	if out.IsError {
		t.Fatalf("%s returned IsError: %s", tool, structuredJSON(out))
	}
	var data []byte
	if out.StructuredContent != nil {
		if data, err = json.Marshal(out.StructuredContent); err != nil {
			t.Fatalf("marshal %s result: %v", tool, err)
		}
	} else {
		data = []byte(mcpExtTextContent(t, out))
	}
	v, err := contract.Decode(data)
	if err != nil {
		t.Fatalf("decode %s result: %v\n%s", tool, err, data)
	}
	return v
}

// contractWorld holds the IDs of the objects seeded by seed-contract.rb,
// discovered through the REST API by their fixed names.
type contractWorld struct {
	ids map[string]int
}

var (
	worldOnce sync.Once
	worldVal  *contractWorld
)

// loadContractWorld discovers the seeded world once per test binary. A
// missing piece is always fatal with an actionable message: a stack that
// predates the seed must be recreated, not silently tested with less.
func loadContractWorld(t *testing.T) *contractWorld {
	t.Helper()
	worldOnce.Do(func() { worldVal = discoverContractWorld(t) })
	if worldVal == nil {
		t.Fatal("contract world discovery failed earlier")
	}
	return worldVal
}

func firstWhere(list any, match func(map[string]any) bool) map[string]any {
	items, _ := list.([]any)
	for _, it := range items {
		if m, ok := it.(map[string]any); ok && match(m) {
			return m
		}
	}
	return nil
}

func jsonInt(v any) int {
	n, ok := v.(json.Number)
	if !ok {
		return 0
	}
	i, _ := strconv.Atoi(n.String())
	return i
}

func field(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func discoverContractWorld(t *testing.T) *contractWorld {
	t.Helper()
	w := &contractWorld{ids: map[string]int{}}
	need := func(name string, id int, hint string) {
		if id == 0 {
			t.Fatalf("contract world is missing %s (%s): recreate the stack so e2e/seed-contract.rb runs (make e2e-down e2e-up)", name, hint)
		}
		w.ids[name] = id
	}
	P := "/projects/" + contractProject

	need("project", jsonInt(field(rawGET(t, P+".json"), "project", "id")), "project "+contractProject)

	issues := field(rawGET(t, "/issues.json?project_id="+contractProject+"&status_id=*&limit=100"), "issues")
	for key, subject := range map[string]string{
		"parent": "Contract parent", "child": "Contract child", "grandchild": "Contract grandchild",
		"leaf": "Contract leaf", "private": "Contract private", "closed": "Contract closed",
	} {
		it := firstWhere(issues, func(m map[string]any) bool { return m["subject"] == subject })
		need(key, jsonInt(field(it, "id")), "issue "+subject)
	}

	versions := field(rawGET(t, P+"/versions.json"), "versions")
	v := firstWhere(versions, func(m map[string]any) bool { return m["name"] == "E2E Contract v1" })
	need("version", jsonInt(field(v, "id")), "version E2E Contract v1")

	categories := field(rawGET(t, P+"/issue_categories.json"), "issue_categories")
	c := firstWhere(categories, func(m map[string]any) bool { return m["name"] == "Contract Category" })
	need("category", jsonInt(field(c, "id")), "issue category Contract Category")

	news := field(rawGET(t, P+"/news.json?limit=100"), "news")
	n := firstWhere(news, func(m map[string]any) bool { return m["title"] == "Contract news" })
	need("news", jsonInt(field(n, "id")), "news Contract news")

	users := field(rawGET(t, "/users.json?limit=100&name=e2econtract"), "users")
	u := firstWhere(users, func(m map[string]any) bool { return m["login"] == "e2econtract" })
	need("user", jsonInt(field(u, "id")), "user e2econtract")

	groups := field(rawGET(t, "/groups.json"), "groups")
	g := firstWhere(groups, func(m map[string]any) bool { return m["name"] == "E2E Contract Group" })
	need("group", jsonInt(field(g, "id")), "group E2E Contract Group")

	need("admin", jsonInt(field(rawGET(t, "/users/current.json"), "user", "id")), "current user")

	members := field(rawGET(t, P+"/memberships.json?limit=100"), "memberships")
	m := firstWhere(members, func(m map[string]any) bool { return jsonInt(field(m, "user", "id")) == w.ids["user"] })
	need("membership", jsonInt(field(m, "id")), "membership of e2econtract")
	// /groups.json only lists givable groups; the builtin ones are reachable
	// through the memberships the seed gives them.
	b := firstWhere(members, func(m map[string]any) bool {
		return field(m, "group_non_member") != nil || field(m, "group_anonymous") != nil
	})
	builtin := jsonInt(field(b, "group_non_member", "id"))
	if builtin == 0 {
		builtin = jsonInt(field(b, "group_anonymous", "id"))
	}
	need("builtin_group", builtin, "a membership of the builtin Non member / Anonymous group")

	entries := field(rawGET(t, "/time_entries.json?project_id="+contractProject+"&limit=100"), "time_entries")
	e := firstWhere(entries, func(m map[string]any) bool { return m["comments"] == "contract time on parent" })
	need("timeentry", jsonInt(field(e, "id")), "time entry on the parent issue")

	withAtt := rawGET(t, "/issues/"+strconv.Itoa(w.ids["parent"])+".json?include=attachments")
	atts, _ := field(withAtt, "issue", "attachments").([]any)
	attID := 0
	if len(atts) > 0 {
		attID = jsonInt(field(atts[0], "id"))
	}
	need("attachment", attID, "attachment on the parent issue")

	rels := field(rawGET(t, "/issues/"+strconv.Itoa(w.ids["leaf"])+"/relations.json"), "relations")
	rel := firstWhere(rels, func(m map[string]any) bool { return m["relation_type"] == "precedes" })
	need("relation", jsonInt(field(rel, "id")), "precedes relation on the leaf issue")

	trackers := field(rawGET(t, "/trackers.json"), "trackers")
	need("tracker", jsonInt(field(firstWhere(trackers, func(map[string]any) bool { return true }), "id")), "a tracker")
	roles := field(rawGET(t, "/roles.json"), "roles")
	need("role", jsonInt(field(firstWhere(roles, func(map[string]any) bool { return true }), "id")), "a role")
	cfs := field(rawGET(t, "/custom_fields.json"), "custom_fields")
	cf := firstWhere(cfs, func(m map[string]any) bool { return m["name"] == "E2E Contract Issue CF" })
	need("customfield", jsonInt(field(cf, "id")), "custom field E2E Contract Issue CF")
	queries := field(rawGET(t, "/queries.json"), "queries")
	q := firstWhere(queries, func(m map[string]any) bool { return m["name"] == "E2E All Open Issues" })
	need("query", jsonInt(field(q, "id")), "saved query E2E All Open Issues")
	return w
}

var placeholderRe = regexp.MustCompile(`\{([a-z_]+|P)\}`)

// expand substitutes {name} placeholders with world IDs ({P} is the contract
// project identifier).
func (w *contractWorld) expand(s string) string {
	return placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		if name == "P" {
			return contractProject
		}
		id, ok := w.ids[name]
		if !ok {
			panic("unknown contract placeholder " + m)
		}
		return strconv.Itoa(id)
	})
}

func (w *contractWorld) id(name string) int {
	id, ok := w.ids[name]
	if !ok {
		panic("unknown contract world id " + name)
	}
	return id
}

func (w *contractWorld) expandAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = w.expand(s)
	}
	return out
}

// dumpContractArtifacts writes the three payloads of a failed case to
// e2e/_artifacts so CI can upload them and a developer can diff them.
func dumpContractArtifacts(t *testing.T, name string, payloads map[string]any) {
	t.Helper()
	dir := filepath.Join(repoRootFromCaller(), "e2e", "_artifacts", getenvDefault("REDMINE_E2E_VERSION", "unknown"), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("artifacts: %v", err)
		return
	}
	for kind, v := range payloads {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(dir, kind+".json"), data, 0o600)
	}
	t.Logf("payloads written to %s", dir)
}

func contractFail(t *testing.T, what string, res contract.Result) {
	t.Helper()
	t.Errorf("%s differs from what Redmine returned (Redmine %s):\n%s\nfix: add the field to internal/models/ (or drop omitempty). "+
		"Only deliberate reshaping belongs in the case's allow rules, each with a reason.",
		what, getenvDefault("REDMINE_E2E_VERSION", "?"), res.String())
}
