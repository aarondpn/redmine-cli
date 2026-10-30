//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil/contract"
)

// contractMCP describes how a case is exercised over MCP. Args receives the
// world so IDs can be filled in; Key names the list inside the tool result
// when it differs from the case's Root.
type contractMCP struct {
	Tool string
	Args func(w *contractWorld) map[string]any
	Key  string
}

// contractCase is one resource the CLI (and, where a tool exists, the MCP
// server) must reproduce faithfully.
type contractCase struct {
	Name string
	// Raw is the Redmine path plus query. Each case asks for a superset
	// include list: Redmine ignores include names it does not know, so one
	// list serves all four supported lines.
	Raw string
	// Root is the key of the payload inside Redmine's envelope; a dotted path
	// descends into nested objects.
	Root string
	// List marks Root as an array. The CLI prints lists as bare arrays, so
	// the envelope (total_count, offset, limit) is only compared for MCP.
	List bool
	// Pick selects the element with this world ID out of a raw list: the
	// CLI's tracker/custom-field/query `get` have no show endpoint behind
	// them and filter the list instead.
	Pick string
	CLI  []string
	MCP  *contractMCP

	AllowCLI, AllowMCP []contract.Rule
}

const (
	issueIncludes   = "children,attachments,relations,changesets,journals,watchers,allowed_statuses,reactions"
	projectIncludes = "trackers,issue_categories,enabled_modules,time_entry_activities,issue_custom_fields"
	userIncludes    = "memberships,groups,auth_source"
)

func splitList(s string) []string { return strings.Split(s, ",") }

// redactedAPIKey is the one deliberate difference on the MCP side: the server
// strips api_key from every result (internal/mcpserver/redact.go). Absent
// makes the diff assert the redaction still happens.
var redactedAPIKey = contract.Rule{
	Path: "**.api_key", Kind: contract.KindMissing, Absent: true,
	Reason: "MCP redacts the API key from every tool result by design",
}

func idArgs(key, world string, extra map[string]any) func(w *contractWorld) map[string]any {
	return func(w *contractWorld) map[string]any {
		m := map[string]any{key: w.id(world)}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
}

func projArgs(extra map[string]any) func(w *contractWorld) map[string]any {
	return func(*contractWorld) map[string]any {
		m := map[string]any{"project_id": contractProject}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
}

func literal(m map[string]any) func(w *contractWorld) map[string]any {
	return func(*contractWorld) map[string]any { return m }
}

// contractCases is the inventory. Resources whose CLI command or MCP tool
// does not exist are simply absent from the relevant surface.
var contractCases = []contractCase{
	{
		Name: "issues_get_full", Raw: "/issues/{parent}.json?include=" + issueIncludes, Root: "issue",
		CLI: []string{"issues", "get", "{parent}", "--include", issueIncludes},
		MCP: &contractMCP{Tool: "get_issue", Args: idArgs("id", "parent", map[string]any{"includes": splitList(issueIncludes)})},
	},
	{
		Name: "issues_get_leaf", Raw: "/issues/{leaf}.json?include=children,relations,journals,watchers", Root: "issue",
		CLI: []string{"issues", "get", "{leaf}", "--include", "children,relations,journals,watchers"},
		MCP: &contractMCP{Tool: "get_issue", Args: idArgs("id", "leaf", map[string]any{"includes": []string{"children", "relations", "journals", "watchers"}})},
	},
	{
		Name: "issues_get_closed", Raw: "/issues/{closed}.json?include=journals", Root: "issue",
		CLI: []string{"issues", "get", "{closed}", "--include", "journals"},
		MCP: &contractMCP{Tool: "get_issue", Args: idArgs("id", "closed", map[string]any{"includes": []string{"journals"}})},
	},
	{
		Name: "issues_list", Raw: "/issues.json?project_id={P}&status_id=*&sort=id&limit=100&include=attachments,relations", Root: "issues", List: true,
		CLI: []string{"issues", "list", "--project", "{P}", "--status", "*", "--sort", "id", "--limit", "100", "--include", "attachments,relations"},
		MCP: &contractMCP{Tool: "list_issues", Args: projArgs(map[string]any{"status_id": "*", "sort": "id", "limit": 100, "includes": []string{"attachments", "relations"}})},
	},
	{
		Name: "projects_get", Raw: "/projects/{P}.json?include=" + projectIncludes, Root: "project",
		CLI: []string{"projects", "get", "{P}", "--include", projectIncludes},
		MCP: &contractMCP{Tool: "get_project", Args: literal(map[string]any{"identifier": contractProject, "includes": splitList(projectIncludes)})},
	},
	{
		Name: "projects_list", Raw: "/projects.json?limit=100&include=" + projectIncludes, Root: "projects", List: true,
		CLI: []string{"projects", "list", "--limit", "100", "--include", projectIncludes},
		MCP: &contractMCP{Tool: "list_projects", Args: literal(map[string]any{"limit": 100, "includes": splitList(projectIncludes)})},
	},
	{
		Name: "versions_list", Raw: "/projects/{P}/versions.json", Root: "versions", List: true,
		CLI: []string{"versions", "list", "--project", "{P}", "--limit", "100"},
		MCP: &contractMCP{Tool: "list_versions", Args: projArgs(map[string]any{"limit": 100})},
	},
	{
		Name: "versions_get", Raw: "/versions/{version}.json", Root: "version",
		CLI: []string{"versions", "get", "{version}"},
		MCP: &contractMCP{Tool: "get_version", Args: idArgs("id", "version", nil)},
	},
	{
		Name: "categories_get", Raw: "/issue_categories/{category}.json", Root: "issue_category",
		CLI: []string{"categories", "get", "{category}"},
		MCP: &contractMCP{Tool: "get_issue_category", Args: idArgs("id", "category", nil)},
	},
	{
		Name: "time_entries_list", Raw: "/time_entries.json?project_id={P}&limit=100", Root: "time_entries", List: true,
		CLI: []string{"time", "list", "--project", "{P}", "--limit", "100"},
		MCP: &contractMCP{Tool: "list_time_entries", Args: projArgs(map[string]any{"limit": 100})},
	},
	{
		Name: "time_entries_get", Raw: "/time_entries/{timeentry}.json", Root: "time_entry",
		CLI: []string{"time", "get", "{timeentry}"},
		MCP: &contractMCP{Tool: "get_time_entry", Args: idArgs("id", "timeentry", nil)},
	},
	{
		Name: "users_list", Raw: "/users.json?limit=100&include=auth_source", Root: "users", List: true,
		CLI: []string{"users", "list", "--limit", "100", "--include", "auth_source"},
		MCP: &contractMCP{Tool: "list_users", Args: literal(map[string]any{"limit": 100, "includes": []string{"auth_source"}})},
	},
	{
		Name: "users_get_seeded", Raw: "/users/{user}.json?include=" + userIncludes, Root: "user",
		CLI:      []string{"users", "get", "{user}", "--include", userIncludes},
		MCP:      &contractMCP{Tool: "get_user", Args: idArgs("id", "user", map[string]any{"includes": splitList(userIncludes)})},
		AllowMCP: []contract.Rule{redactedAPIKey},
	},
	{
		Name: "users_get_admin", Raw: "/users/{admin}.json?include=" + userIncludes, Root: "user",
		CLI:      []string{"users", "get", "{admin}", "--include", userIncludes},
		MCP:      &contractMCP{Tool: "get_user", Args: idArgs("id", "admin", map[string]any{"includes": splitList(userIncludes)})},
		AllowMCP: []contract.Rule{redactedAPIKey},
	},
	{
		Name: "users_current", Raw: "/users/current.json?include=memberships,groups", Root: "user",
		CLI:      []string{"users", "me", "--include", "memberships,groups"},
		MCP:      &contractMCP{Tool: "me", Args: literal(map[string]any{"includes": []string{"memberships", "groups"}})},
		AllowMCP: []contract.Rule{redactedAPIKey},
	},
	{
		Name: "my_account", Raw: "/my/account.json", Root: "user",
		CLI:      []string{"my-account", "get"},
		MCP:      &contractMCP{Tool: "get_my_account", Args: literal(map[string]any{})},
		AllowMCP: []contract.Rule{redactedAPIKey},
	},
	{
		Name: "groups_list", Raw: "/groups.json", Root: "groups", List: true,
		CLI: []string{"groups", "list", "--limit", "100"},
		MCP: &contractMCP{Tool: "list_groups", Args: literal(map[string]any{"limit": 100})},
	},
	{
		Name: "groups_get", Raw: "/groups/{group}.json?include=users,memberships", Root: "group",
		CLI: []string{"groups", "get", "{group}", "--include", "users,memberships"},
		MCP: &contractMCP{Tool: "get_group", Args: idArgs("id", "group", map[string]any{"includes": []string{"users", "memberships"}})},
	},
	{
		Name: "groups_get_builtin", Raw: "/groups/{builtin_group}.json?include=users,memberships", Root: "group",
		CLI: []string{"groups", "get", "{builtin_group}", "--include", "users,memberships"},
		MCP: &contractMCP{Tool: "get_group", Args: idArgs("id", "builtin_group", map[string]any{"includes": []string{"users", "memberships"}})},
	},
	{
		Name: "memberships_list", Raw: "/projects/{P}/memberships.json?limit=100", Root: "memberships", List: true,
		CLI: []string{"memberships", "list", "--project", "{P}", "--limit", "100"},
		MCP: &contractMCP{Tool: "list_memberships", Args: projArgs(map[string]any{"limit": 100})},
	},
	{
		Name: "memberships_get", Raw: "/memberships/{membership}.json", Root: "membership",
		CLI: []string{"memberships", "get", "{membership}"},
		MCP: &contractMCP{Tool: "get_membership", Args: idArgs("id", "membership", nil)},
	},
	{
		Name: "roles_list", Raw: "/roles.json", Root: "roles", List: true,
		CLI: []string{"roles", "list"},
		MCP: &contractMCP{Tool: "list_roles", Args: literal(map[string]any{})},
	},
	{
		Name: "roles_get", Raw: "/roles/{role}.json", Root: "role",
		CLI: []string{"roles", "get", "{role}"},
		MCP: &contractMCP{Tool: "get_role", Args: idArgs("id", "role", nil)},
	},
	{
		Name: "trackers_list", Raw: "/trackers.json", Root: "trackers", List: true,
		CLI: []string{"trackers", "list"},
		MCP: &contractMCP{Tool: "list_trackers", Args: literal(map[string]any{})},
	},
	{
		Name: "trackers_get", Raw: "/trackers.json", Root: "trackers", Pick: "tracker",
		CLI: []string{"trackers", "get", "{tracker}"},
		MCP: &contractMCP{Tool: "get_tracker", Args: idArgs("id", "tracker", nil)},
	},
	{
		Name: "statuses_list", Raw: "/issue_statuses.json", Root: "issue_statuses", List: true,
		CLI: []string{"statuses", "list"},
		MCP: &contractMCP{Tool: "list_statuses", Args: literal(map[string]any{})},
	},
	{
		Name: "custom_fields_list", Raw: "/custom_fields.json", Root: "custom_fields", List: true,
		CLI: []string{"custom-fields", "list"},
		MCP: &contractMCP{Tool: "list_custom_fields", Args: literal(map[string]any{})},
	},
	{
		Name: "custom_fields_get", Raw: "/custom_fields.json", Root: "custom_fields", Pick: "customfield",
		CLI: []string{"custom-fields", "get", "{customfield}"},
		MCP: &contractMCP{Tool: "get_custom_field", Args: idArgs("id", "customfield", nil)},
	},
	{
		Name: "queries_list", Raw: "/queries.json", Root: "queries", List: true,
		CLI: []string{"queries", "list"},
		MCP: &contractMCP{Tool: "list_queries", Args: literal(map[string]any{})},
	},
	{
		Name: "queries_get", Raw: "/queries.json", Root: "queries", Pick: "query",
		CLI: []string{"queries", "get", "{query}"},
	},
	{
		Name: "search_issues", Raw: "/search.json?q=Contract&issues=1&limit=100", Root: "results", List: true,
		CLI: []string{"search", "issues", "Contract", "--limit", "100"},
		MCP: &contractMCP{Tool: "search", Args: literal(map[string]any{"query": "Contract", "issues": true, "limit": 100})},
	},
	{
		Name: "search_wiki", Raw: "/search.json?q=Contract&wiki_pages=1&limit=100", Root: "results", List: true,
		CLI: []string{"search", "wiki", "Contract", "--limit", "100"},
		MCP: &contractMCP{Tool: "search", Args: literal(map[string]any{"query": "Contract", "wiki_pages": true, "limit": 100})},
	},
	{
		Name: "search_projects", Raw: "/search.json?q=Contract&projects=1&limit=100", Root: "results", List: true,
		CLI: []string{"search", "projects", "Contract", "--limit", "100"},
		MCP: &contractMCP{Tool: "search", Args: literal(map[string]any{"query": "Contract", "projects": true, "limit": 100})},
	},
	{
		Name: "wiki_get", Raw: "/projects/{P}/wiki/Contract_Child.json?include=attachments", Root: "wiki_page",
		CLI: []string{"wiki", "get", "Contract_Child", "--project", "{P}", "--include", "attachments"},
		MCP: &contractMCP{Tool: "get_wiki_page", Args: projArgs(map[string]any{"page": "Contract_Child", "includes": []string{"attachments"}})},
	},
	{
		Name: "wiki_list", Raw: "/projects/{P}/wiki/index.json", Root: "wiki_pages", List: true,
		CLI: []string{"wiki", "list", "--project", "{P}", "--limit", "100"},
		MCP: &contractMCP{Tool: "list_wiki_pages", Args: projArgs(map[string]any{"limit": 100}), Key: "pages"},
	},
	{
		Name: "files_list", Raw: "/projects/{P}/files.json", Root: "files", List: true,
		CLI: []string{"files", "list", "--project", "{P}", "--limit", "100"},
		MCP: &contractMCP{Tool: "list_project_files", Args: projArgs(map[string]any{"limit": 100})},
	},
	{
		Name: "attachments_get", Raw: "/attachments/{attachment}.json", Root: "attachment",
		CLI: []string{"attachments", "get", "{attachment}"},
		MCP: &contractMCP{Tool: "get_attachment", Args: idArgs("id", "attachment", nil)},
	},
	{
		Name: "categories_list", Raw: "/projects/{P}/issue_categories.json", Root: "issue_categories", List: true,
		CLI: []string{"categories", "list", "--project", "{P}"},
		MCP: &contractMCP{Tool: "list_categories", Args: projArgs(nil)},
	},
	{
		Name: "relations_list", Raw: "/issues/{leaf}/relations.json", Root: "relations", List: true,
		CLI: []string{"issues", "relations", "list", "{leaf}"},
		MCP: &contractMCP{Tool: "list_issue_relations", Args: idArgs("issue_id", "leaf", nil)},
	},
	{
		Name: "enumerations_activities", Raw: "/enumerations/time_entry_activities.json", Root: "time_entry_activities", List: true,
		CLI: []string{"activities", "list"},
		MCP: &contractMCP{Tool: "list_time_entry_activities", Args: literal(map[string]any{})},
	},
	{
		Name: "priorities_list", Raw: "/enumerations/issue_priorities.json", Root: "issue_priorities", List: true,
		CLI: []string{"priorities", "list"},
		MCP: &contractMCP{Tool: "list_priorities", Args: literal(map[string]any{})},
	},
	{
		// CLI only: documents are not supported elsewhere, so no MCP tool.
		Name: "document_categories_list", Raw: "/enumerations/document_categories.json", Root: "document_categories", List: true,
		CLI: []string{"document-categories", "list"},
	},
	{
		// Project-specific activities (overrides have their own IDs) only come
		// from the project payload; the enumerations endpoint ignores project_id.
		Name: "activities_list_project", Raw: "/projects/{P}.json?include=time_entry_activities", Root: "project.time_entry_activities", List: true,
		CLI: []string{"activities", "list", "--project", "{P}"},
		MCP: &contractMCP{Tool: "list_time_entry_activities", Args: projArgs(nil), Key: "time_entry_activities"},
	},
	{
		Name: "relations_get", Raw: "/relations/{relation}.json", Root: "relation",
		CLI: []string{"issues", "relations", "get", "{relation}"},
		MCP: &contractMCP{Tool: "get_issue_relation", Args: idArgs("id", "relation", nil)},
	},
}

// knownZeroDrops are zero values ("", 0, false) the models have always
// dropped through omitempty, so `-o json` and MCP omit the key where Redmine
// sends it. Decoding fields Redmine sends and the models lack is what
// issue #202 covers; the omitempty behaviour of fields that already existed
// is a separate, pre-existing output convention ("absent means false"), kept
// as it is and listed here so any other zero value that gets dropped is
// still reported. Remove an entry when its omitempty goes away.
var knownZeroDrops = func() []contract.Rule {
	var rules []contract.Rule
	for _, p := range []string{
		"homepage", "inherit_members", "regexp", "comments",
	} {
		rules = append(rules, contract.Rule{
			Path: "**." + p, Kind: contract.KindZeroDrop, Optional: true,
			Reason: "pre-existing omitempty: a zero " + p + " is omitted from CLI/MCP JSON",
		})
	}
	return rules
}()

// contractNoCase lists read-only MCP tools that have no contract case, with
// the reason. TestContract_EveryReadToolHasACase fails for a read tool in
// neither place, so a new tool cannot skip the contract.
var contractNoCase = map[string]string{
	"list_issue_watchers":  "watchers are covered by the issues_get_* cases (include=watchers)",
	"list_project_members": "alias view of the memberships already covered by memberships_list",
	"summary_time_entries": "computed aggregate, not a Redmine payload",
}

// rawRoot extracts the part of the Redmine payload a case compares.
func (c contractCase) rawRoot(t *testing.T, w *contractWorld, doc any) any {
	t.Helper()
	// Root may be dotted ("project.time_entry_activities") to reach into a
	// nested envelope.
	root := field(doc, strings.Split(c.Root, ".")...)
	if root == nil {
		t.Fatalf("raw %s has no %q key", c.Name, c.Root)
	}
	if c.Pick == "" {
		return root
	}
	want := w.id(c.Pick)
	it := firstWhere(root, func(m map[string]any) bool { return jsonInt(m["id"]) == want })
	if it == nil {
		t.Fatalf("raw %s has no element with id %d", c.Name, want)
	}
	return it
}

func TestContract(t *testing.T) {
	requireE2E(t)
	w := loadContractWorld(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	session := r.startMCPStdio(t, ctx)

	for _, c := range contractCases {
		t.Run(c.Name, func(t *testing.T) {
			doc := rawGET(t, w.expand(c.Raw))
			raw := c.rawRoot(t, w, doc)
			payloads := map[string]any{"raw": raw}

			if len(c.CLI) > 0 {
				t.Run("cli", func(t *testing.T) {
					got := r.runJSONAny(t, w.expandAll(c.CLI)...)
					payloads["cli"] = got
					if res := contract.Diff(raw, got, append(c.AllowCLI[:len(c.AllowCLI):len(c.AllowCLI)], knownZeroDrops...)); !res.OK() {
						contractFail(t, "CLI -o json", res)
						dumpContractArtifacts(t, c.Name, payloads)
					}
				})
			}

			if c.MCP != nil {
				t.Run("mcp", func(t *testing.T) {
					res := mcpCallAny(t, ctx, session, c.MCP.Tool, c.MCP.Args(w))
					got := res
					if c.List {
						key := c.MCP.Key
						if key == "" {
							key = c.Root
						}
						got = field(res, key)
						// The MCP envelope carries total_count (and for search
						// also offset/limit); the CLI drops it by design.
						meta := map[string]any{}
						gotMeta := map[string]any{}
						for _, k := range []string{"total_count", "offset", "limit"} {
							if field(doc, k) != nil && (k == "total_count" || field(res, k) != nil) {
								meta[k] = field(doc, k)
								gotMeta[k] = field(res, k)
							}
						}
						if mres := contract.Diff(meta, gotMeta, nil); !mres.OK() {
							contractFail(t, "MCP "+c.MCP.Tool+" envelope", mres)
						}
					}
					payloads["mcp"] = got
					if dres := contract.Diff(raw, got, append(c.AllowMCP[:len(c.AllowMCP):len(c.AllowMCP)], knownZeroDrops...)); !dres.OK() {
						contractFail(t, "MCP "+c.MCP.Tool, dres)
						dumpContractArtifacts(t, c.Name, payloads)
					}
				})
			}
		})
	}
}
