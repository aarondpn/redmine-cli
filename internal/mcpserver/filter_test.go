package mcpserver

import (
	"context"
	"net/http"
	"testing"
)

func TestResolveOptions_NoSpecKeepsDefaults(t *testing.T) {
	got := ResolveOptions(Options{}, FilterSpec{})
	if got.EnabledGroups != nil {
		t.Errorf("expected nil EnabledGroups, got %v", got.EnabledGroups)
	}
	if got.EnabledTools != nil || got.DisabledTools != nil {
		t.Errorf("expected nil tool maps, got allow=%v deny=%v", got.EnabledTools, got.DisabledTools)
	}
}

func TestResolveOptions_EnableGroupsNarrowsSet(t *testing.T) {
	got := ResolveOptions(Options{}, FilterSpec{EnableGroups: []Group{GroupIssues, GroupWiki}})
	want := map[Group]bool{GroupIssues: true, GroupWiki: true}
	if len(got.EnabledGroups) != len(want) {
		t.Fatalf("len = %d, want %d", len(got.EnabledGroups), len(want))
	}
	for g := range want {
		if !got.EnabledGroups[g] {
			t.Errorf("group %q expected enabled", g)
		}
	}
}

func TestResolveOptions_DisableGroupsSubtractsFromAll(t *testing.T) {
	got := ResolveOptions(Options{}, FilterSpec{DisableGroups: []Group{GroupTime}})
	if got.EnabledGroups[GroupTime] {
		t.Error("time group should be disabled")
	}
	if !got.EnabledGroups[GroupIssues] {
		t.Error("issues should remain enabled when only time is disabled")
	}
}

func TestResolveOptions_DisableSubtractsFromEnable(t *testing.T) {
	got := ResolveOptions(Options{}, FilterSpec{
		EnableGroups:  []Group{GroupIssues, GroupWiki},
		DisableGroups: []Group{GroupWiki},
	})
	if !got.EnabledGroups[GroupIssues] {
		t.Error("issues should remain enabled")
	}
	if got.EnabledGroups[GroupWiki] {
		t.Error("wiki should be subtracted by disable list")
	}
}

func TestParseGroup_RejectsUnknown(t *testing.T) {
	if _, err := ParseGroup("nonsense"); err == nil {
		t.Fatal("expected error for unknown group")
	}
}

func TestParseGroup_AcceptsCanonical(t *testing.T) {
	for _, g := range AllGroups() {
		got, err := ParseGroup(string(g))
		if err != nil {
			t.Errorf("ParseGroup(%q): %v", g, err)
		}
		if got != g {
			t.Errorf("ParseGroup(%q) = %q, want %q", g, got, g)
		}
	}
}

func TestGroupFilter_HidesDisabledGroup(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	opts := ResolveOptions(Options{Version: "v0", EnableWrites: true}, FilterSpec{
		EnableGroups: []Group{GroupIssues},
	})
	cs, cleanup := newConnectedSession(t, apiClient, opts)
	defer cleanup()

	names := listToolNames(t, cs)
	mustHave := []string{"list_issues", "get_issue", "create_issue"}
	mustMiss := []string{"list_projects", "list_users", "search", "list_wiki_pages", "list_versions", "list_groups"}

	for _, n := range mustHave {
		if !contains(names, n) {
			t.Errorf("issues tool %q missing when only issues group enabled", n)
		}
	}
	for _, n := range mustMiss {
		if contains(names, n) {
			t.Errorf("non-issue tool %q registered when only issues group enabled", n)
		}
	}
}

func TestToolDenyList_HidesIndividualTool(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	opts := ResolveOptions(Options{Version: "v0", EnableWrites: true}, FilterSpec{
		DisableTools: []string{"delete_issue", "delete_project"},
	})
	cs, cleanup := newConnectedSession(t, apiClient, opts)
	defer cleanup()

	names := listToolNames(t, cs)
	if contains(names, "delete_issue") {
		t.Error("delete_issue should be hidden by deny-list")
	}
	if contains(names, "delete_project") {
		t.Error("delete_project should be hidden by deny-list")
	}
	if !contains(names, "create_issue") {
		t.Error("create_issue should still be present (not in deny list)")
	}
	if !contains(names, "delete_user") {
		t.Error("delete_user should still be present (not in deny list)")
	}
}

func TestToolAllowList_OnlyListedToolsRegistered(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	opts := ResolveOptions(Options{Version: "v0"}, FilterSpec{
		EnableTools: []string{"list_issues", "get_issue"},
	})
	cs, cleanup := newConnectedSession(t, apiClient, opts)
	defer cleanup()

	names := listToolNames(t, cs)
	for _, n := range []string{"list_issues", "get_issue"} {
		if !contains(names, n) {
			t.Errorf("allow-listed tool %q missing", n)
		}
	}
	for _, n := range []string{"list_projects", "list_versions", "search", "me"} {
		if contains(names, n) {
			t.Errorf("tool %q outside allow-list was registered", n)
		}
	}
}

func TestResourceTemplatesGatedByGroup(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	opts := ResolveOptions(Options{Version: "v0"}, FilterSpec{
		EnableGroups: []Group{GroupIssues},
	})
	cs, cleanup := newConnectedSession(t, apiClient, opts)
	defer cleanup()

	res, err := cs.ListResourceTemplates(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}

	templates := make(map[string]bool, len(res.ResourceTemplates))
	for _, rt := range res.ResourceTemplates {
		templates[rt.URITemplate] = true
	}

	if !templates[tmplIssue] {
		t.Error("issue resource template should remain when issues group is enabled")
	}
	for _, t2 := range []string{tmplProject, tmplUser, tmplTimeEntry, tmplWiki, tmplVersion} {
		if templates[t2] {
			t.Errorf("resource template %q should be gated off when its group is disabled", t2)
		}
	}
}

func TestAllToolsCatalogCoversEveryGroup(t *testing.T) {
	byGroup := ToolsByGroup()
	if len(byGroup) != len(AllGroups()) {
		t.Fatalf("ToolsByGroup len=%d, want %d", len(byGroup), len(AllGroups()))
	}
	for _, g := range AllGroups() {
		if _, ok := byGroup[g]; !ok {
			t.Errorf("group %q missing from catalog", g)
		}
	}

	// Spot-check that at least one well-known tool ended up in each expected group.
	wantOne := map[Group]string{
		GroupIssues:      "list_issues",
		GroupProjects:    "list_projects",
		GroupTime:        "list_time_entries",
		GroupUsers:       "list_users",
		GroupGroups:      "list_groups",
		GroupSearch:      "search",
		GroupMeta:        "list_versions",
		GroupWiki:        "list_wiki_pages",
		GroupMemberships: "list_memberships",
	}
	for g, name := range wantOne {
		found := false
		for _, d := range byGroup[g] {
			if d.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in group %q catalog", name, g)
		}
	}
}

func TestOptionsToolEnabled(t *testing.T) {
	opts := Options{
		EnabledTools:  map[string]bool{"list_issues": true},
		DisabledTools: map[string]bool{"get_issue": true},
	}
	if !opts.toolEnabled("list_issues") {
		t.Error("list_issues should be enabled by allow-list")
	}
	if opts.toolEnabled("create_issue") {
		t.Error("create_issue not in allow-list should be hidden")
	}
	if opts.toolEnabled("get_issue") {
		t.Error("get_issue should be hidden by deny-list (overrides allow)")
	}

	noFilter := Options{}
	if !noFilter.toolEnabled("anything") {
		t.Error("nil filters should pass through")
	}
}
