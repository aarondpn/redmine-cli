//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil/contract"
)

// gatedKey is one row of the central table of version-gated JSON keys.
//
// The contract diff (TestContract) needs no version knowledge: whatever
// Redmine sends must reach the output. Its blind spot is a key Redmine never
// sends because the fixture does not trigger it, which passes vacuously.
// TestContract_Coverage closes that hole from both sides:
//
//   - on Redmine >= Since the raw payload must contain a non-null value at
//     Path, so the seeded world really exercises the key;
//   - on Redmine < Since it must not, so the table cannot drift away from
//     what upstream actually ships.
//
// This table is the one place that records which Redmine release introduced
// which key. Do not assert "the JSON has key X" behind an inline
// redmineAtLeast in a test; add a row here instead.
type gatedKey struct {
	Case  string // contractCase.Name the raw payload is taken from
	Path  string // path below the case root, "[]" fans out over arrays
	Since [2]int // first Redmine line that sends the key; {0, 0} = every supported line
	Ref   string // upstream reference
	Cond  string // fixture condition that makes Redmine emit it (see seed-contract.rb)
}

var always = [2]int{0, 0}

var gatedKeys = []gatedKey{
	// Issues.
	{"issues_get_full", "status.is_closed", [2]int{5, 0}, "redmine.org/issues/15855", ""},
	{"issues_get_closed", "closed_on", always, "since Redmine 2.3", "a closed issue"},
	{"issues_get_full", "spent_hours", always, "", "time entries on the issue, view_time_entries"},
	{"issues_get_full", "total_spent_hours", always, "since Redmine 3.3", "time entries on a child issue"},
	{"issues_get_full", "total_estimated_hours", always, "since Redmine 3.3", "estimated hours on the children"},
	{"issues_get_full", "children[].children[].subject", always, "issues_helper#render_api_issue_children", "three-level issue tree"},
	{"issues_get_full", "changesets[].revision", always, "", "a changeset linked to the issue"},
	{"issues_get_full", "allowed_statuses[].id", [2]int{5, 0}, "redmine.org/issues/24976", ""},
	{"issues_get_full", "journals[].updated_on", [2]int{6, 0}, "redmine.org/issues/40449", "an edited journal note"},
	{"issues_get_full", "journals[].updated_by.id", [2]int{6, 0}, "redmine.org/issues/40449", "an edited journal note"},
	{"issues_get_full", "reactions[].user.id", [2]int{7, 1}, "redmine.org/issues/44288 (render_api_reactions is absent from Redmine 7.0.x; Since 7.1 is a guess at the next release, re-check when a newer image joins the matrix)", "a reaction on the issue"},
	{"issues_get_full", "journals[].reactions[].user.id", [2]int{7, 1}, "redmine.org/issues/44288 (render_api_reactions is absent from Redmine 7.0.x; Since 7.1 is a guess at the next release, re-check when a newer image joins the matrix)", "a reaction on a journal"},
	{"issues_list", "[].spent_hours", [2]int{5, 0}, "redmine.org/issues/34857", ""},
	{"issues_list", "[].total_spent_hours", [2]int{5, 0}, "redmine.org/issues/34857", ""},
	{"issues_list", "[].total_estimated_hours", [2]int{5, 0}, "redmine.org/issues/34857", ""},

	// Versions and time entries.
	{"versions_get", "estimated_hours", always, "redmine.org/issues/32242", "estimated hours on the version's issues"},
	{"versions_get", "spent_hours", always, "redmine.org/issues/32242", "time entries on the version's issues"},
	{"versions_get", "custom_fields[].value", always, "", "a version custom field value"},
	{"versions_list", "[].custom_fields[].value", always, "", "a version custom field value"},
	{"time_entries_list", "[].custom_fields[].value", always, "", "a time entry custom field value"},
	{"time_entries_get", "custom_fields[].value", always, "", "a time entry custom field value"},

	// Users.
	{"users_get_seeded", "passwd_changed_on", always, "redmine.org/issues/33592", "update_column on the seeded user"},
	{"users_get_seeded", "twofa_scheme", always, "redmine.org/issues/34242", "twofa_scheme set on the seeded user"},
	{"users_get_seeded", "avatar_url", [2]int{5, 0}, "redmine.org/issues/36303", "Setting.gravatar_enabled"},
	{"users_get_seeded", "auth_source.name", [2]int{6, 0}, "redmine.org/issues/23307", "an LDAP auth source on the user, include=auth_source, admin"},
	{"users_get_seeded", "memberships[].roles[].inherited", always, "", "membership through a group"},
	{"users_get_seeded", "groups[].name", always, "", "include=groups"},
	{"users_get_seeded", "custom_fields[].value", always, "", "a user custom field value"},

	// Groups and memberships.
	{"groups_get", "custom_fields[].value", always, "", "a group custom field value"},
	{"groups_get_builtin", "builtin", always, "", "a builtin group"},
	{"memberships_list", "[].group_non_member.id", always, "members/index.api.rsb", "a project membership of the builtin Non member group"},
	{"memberships_list", "[].group_anonymous.id", always, "members/index.api.rsb", "a project membership of the builtin Anonymous group"},
	{"memberships_list", "[].roles[].inherited", always, "", "a user whose role comes from a group"},

	// Metadata.
	{"statuses_list", "[].description", [2]int{5, 1}, "redmine.org/issues/2568", "a description on an issue status"},
	{"custom_fields_list", "[].is_for_all", [2]int{7, 0}, "redmine.org/issues/44153", ""},
	{"custom_fields_list", "[].roles[].id", [2]int{7, 0}, "redmine.org/issues/44152", "the role-restricted time entry custom field"},
	{"wiki_get", "project.id", [2]int{7, 0}, "redmine.org/issues/43569", ""},
	{"wiki_get", "parent.title", always, "", "a wiki page with a parent"},
	{"enumerations_activities", "[].custom_fields[].value", always, "enumerations/index.api.rsb", "an activity custom field value"},
}

// TestContract_Coverage asserts, for every gated key, that the seeded world
// makes Redmine emit it exactly on the releases the table says.
func TestContract_Coverage(t *testing.T) {
	requireE2E(t)
	if _, _, ok := parseRedmineVersion(e2eVersion()); !ok {
		if os.Getenv("CI") == "true" {
			t.Fatalf("REDMINE_E2E_VERSION is unset or unparseable (%q): CI must pin the version so the coverage half of the contract runs", e2eVersion())
		}
		skipIfVersionUnknown(t, "version-gated key coverage")
	}
	w := loadContractWorld(t)

	byName := map[string]contractCase{}
	for _, c := range contractCases {
		byName[c.Name] = c
	}
	raws := map[string]any{}

	for _, g := range gatedKeys {
		t.Run(g.Case+"/"+g.Path, func(t *testing.T) {
			c, ok := byName[g.Case]
			if !ok {
				t.Fatalf("gate table references unknown case %q", g.Case)
			}
			root, ok := raws[g.Case]
			if !ok {
				root = c.rawRoot(t, w, rawGET(t, w.expand(c.Raw)))
				raws[g.Case] = root
			}
			found := len(contract.Get(root, g.Path)) > 0
			switch {
			case redmineAtLeast(g.Since[0], g.Since[1]) && !found:
				t.Fatalf("Redmine %s sends no non-null %s in %s although the table says it does since %d.%d (%s).\n"+
					"Either the fixture does not trigger it (%s; see e2e/seed-contract.rb), in which case the contract diff passes vacuously, "+
					"or Since is wrong.", e2eVersion(), g.Path, g.Case, g.Since[0], g.Since[1], g.Ref, g.Cond)
			case !redmineAtLeast(g.Since[0], g.Since[1]) && found:
				t.Fatalf("Redmine %s already sends %s in %s although the table says it arrives with %d.%d (%s): lower Since.",
					e2eVersion(), g.Path, g.Case, g.Since[0], g.Since[1], g.Ref)
			}
		})
	}
}

// TestContract_Enumerations covers the one resource without a CLI or MCP
// surface: the enumerations are only reachable through the API client, so the
// models it decodes into are diffed against the raw payload directly.
func TestContract_Enumerations(t *testing.T) {
	requireE2E(t)
	cfg := &config.Config{Server: e2eBaseURL(), APIKey: e2eAPIKey(), AuthMethod: "apikey"}
	client, err := api.NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := context.Background()

	activities, err := client.Enumerations.TimeEntryActivities(ctx)
	if err != nil {
		t.Fatalf("time entry activities: %v", err)
	}
	priorities, err := client.Enumerations.IssuePriorities(ctx)
	if err != nil {
		t.Fatalf("issue priorities: %v", err)
	}

	for _, tc := range []struct {
		name, path, root string
		got              any
	}{
		{"time_entry_activities", "/enumerations/time_entry_activities.json", "time_entry_activities", activities},
		{"issue_priorities", "/enumerations/issue_priorities.json", "issue_priorities", priorities},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := field(rawGET(t, tc.path), tc.root)
			got, err := contract.Normalize(tc.got)
			if err != nil {
				t.Fatalf("normalize: %v", err)
			}
			if res := contract.Diff(raw, got, nil); !res.OK() {
				contractFail(t, "enumerations", res)
			}
		})
	}
}

// TestContract_EveryReadToolHasACase fails when the read-only MCP tool
// catalogue contains a tool neither exercised by a contract case nor listed
// in contractNoCase with a reason, so a newly added tool cannot skip the
// contract.
func TestContract_EveryReadToolHasACase(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx)

	covered := map[string]bool{}
	for _, c := range contractCases {
		if c.MCP != nil {
			covered[c.MCP.Tool] = true
		}
	}
	listed := map[string]bool{}
	for tool, err := range session.Tools(ctx, &mcp.ListToolsParams{}) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		listed[tool.Name] = true
		if !covered[tool.Name] && contractNoCase[tool.Name] == "" {
			t.Errorf("read-only MCP tool %q has no contract case: add one to contractCases, or an entry with a reason to contractNoCase", tool.Name)
		}
	}
	for tool := range contractNoCase {
		if !listed[tool] {
			t.Errorf("contractNoCase lists %q which is not a read-only MCP tool any more: remove the entry", tool)
		}
	}
	for tool := range covered {
		if !listed[tool] {
			t.Errorf("a contract case uses MCP tool %q which the read-only catalogue does not offer", tool)
		}
	}
}
