package mcpserver

import (
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

func catalogTools() []ExtensionTool {
	return []ExtensionTool{
		{Extension: "pr", Version: "v1.2.0", Command: &spec.Command{Name: "list", Description: "List PRs"}},
		{Extension: "pr", Version: "v1.2.0", Command: &spec.Command{Name: "link", Description: "Link a PR", Writes: true}},
		{Extension: "ci", Command: &spec.Command{Name: "status", Description: "CI status"}},
		// Not tools: CLI only, raw, and a clash with a core tool.
		{Extension: "ci", Command: &spec.Command{Name: "hidden", Description: "x", MCP: new(bool)}},
		{Extension: "ci", Command: &spec.Command{Name: "shell", Description: "x", Raw: true}},
		{Extension: "list", Command: &spec.Command{Name: "issues", Description: "x"}},
	}
}

func TestCatalog_Groups(t *testing.T) {
	c := NewCatalog(catalogTools())
	groups := c.Groups()
	var tail []string
	for _, g := range groups[len(AllGroups()):] {
		tail = append(tail, string(g))
	}
	// ext-list has no tool left after the clash, so it is not a group.
	if want := "ext-ci,ext-pr,extensions"; strings.Join(tail, ",") != want {
		t.Errorf("extension groups = %v, want %s", tail, want)
	}
	for i, g := range AllGroups() {
		if groups[i] != g {
			t.Errorf("groups[%d] = %q, want %q (canonical order first)", i, groups[i], g)
		}
	}
	if got := c.ExtensionGroups(); len(got) != 2 || got[0] != "ext-ci" || got[1] != "ext-pr" {
		t.Errorf("ExtensionGroups() = %v", got)
	}

	empty := NewCatalog(nil)
	if _, err := empty.ParseGroup("extensions"); err != nil {
		t.Error("the umbrella group must be valid without any extension")
	}
	if len(empty.Groups()) != len(AllGroups())+1 {
		t.Errorf("empty catalog groups = %v", empty.Groups())
	}
}

func TestCatalog_ParseGroup(t *testing.T) {
	c := NewCatalog(catalogTools())
	tests := []struct {
		in      string
		want    Group
		wantErr string
	}{
		{in: "issues", want: GroupIssues},
		{in: " EXT-PR ", want: "ext-pr"},
		{in: "extensions", want: GroupExtensions},
		{in: "ext-nope", wantErr: `no installed extension "nope" provides MCP tools`},
		{in: "ext-list", wantErr: `no installed extension "list"`},
		{in: "nonsense", wantErr: "valid: ext-ci, ext-pr, extensions, groups, issues"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := c.ParseGroup(tt.in)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ParseGroup(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}

	gs, err := c.ParseGroups([]string{"issues", "", " ", "ext-ci"})
	if err != nil || len(gs) != 2 || gs[1] != "ext-ci" {
		t.Errorf("ParseGroups = %v, %v", gs, err)
	}
	if _, err := c.ParseGroups([]string{"issues", "ext-nope"}); err == nil {
		t.Error("ParseGroups should reject an unknown extension group")
	}
}

func TestCatalog_ParseTools(t *testing.T) {
	c := NewCatalog(catalogTools())
	got, err := c.ParseTools([]string{" pr_list ", "", "list_issues", "pr_link"})
	if err != nil {
		t.Fatalf("ParseTools: %v", err)
	}
	if strings.Join(got, ",") != "pr_list,list_issues,pr_link" {
		t.Errorf("ParseTools = %v", got)
	}
	for _, bad := range []string{"ci_hidden", "ci_shell", "pr_nope"} {
		if _, err := c.ParseTools([]string{bad}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
			t.Errorf("ParseTools(%q) err = %v, want unknown tool", bad, err)
		}
	}
	// The package-level functions stay static.
	if IsKnownTool("pr_list") {
		t.Error("package-level IsKnownTool must only know generated tools")
	}
	if _, err := ParseGroup("ext-pr"); err == nil {
		t.Error("package-level ParseGroup must only know canonical groups")
	}
}

func TestCatalog_ToolsByGroup(t *testing.T) {
	c := NewCatalog(catalogTools())
	byGroup := c.ToolsByGroup()
	if len(byGroup) != len(c.Groups()) {
		t.Errorf("ToolsByGroup has %d groups, want %d", len(byGroup), len(c.Groups()))
	}
	names := func(ds []ToolDescriptor) string {
		var out []string
		for _, d := range ds {
			out = append(out, d.Name)
		}
		return strings.Join(out, ",")
	}
	if got := names(byGroup["ext-pr"]); got != "pr_link,pr_list" {
		t.Errorf("ext-pr = %s", got)
	}
	if got := names(byGroup[GroupExtensions]); got != "ci_status,pr_link,pr_list" {
		t.Errorf("extensions = %s", got)
	}
	if len(byGroup[GroupIssues]) != len(ToolsByGroup()[GroupIssues]) {
		t.Error("core groups must keep their generated tools")
	}

	pr := byGroup["ext-pr"]
	want := ToolDescriptor{Name: "pr_list", Description: "List PRs.", Group: "ext-pr", Extension: "pr", ExtensionVersion: "v1.2.0"}
	if pr[1] != want {
		t.Errorf("pr_list = %+v, want %+v", pr[1], want)
	}
	if link := pr[0]; !link.Writes || link.Description != "Link a PR. Requires --enable-writes." {
		t.Errorf("pr_link = %+v", link)
	}
}

func TestResolveOptions_ExtensionGroups(t *testing.T) {
	tools := catalogTools()
	tests := []struct {
		name    string
		spec    FilterSpec
		enabled []Group
		off     []Group
	}{
		{name: "disable umbrella", spec: FilterSpec{DisableGroups: []Group{GroupExtensions}}, enabled: []Group{GroupIssues, GroupWiki}, off: []Group{"ext-pr", "ext-ci"}},
		{name: "disable one extension", spec: FilterSpec{DisableGroups: []Group{"ext-pr"}}, enabled: []Group{GroupIssues, "ext-ci"}, off: []Group{"ext-pr"}},
		{name: "enable umbrella", spec: FilterSpec{EnableGroups: []Group{GroupExtensions}}, enabled: []Group{"ext-pr", "ext-ci"}, off: []Group{GroupIssues}},
		{name: "enable one extension", spec: FilterSpec{EnableGroups: []Group{"ext-ci", GroupWiki}}, enabled: []Group{"ext-ci", GroupWiki}, off: []Group{"ext-pr", GroupIssues}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ResolveOptions(Options{ExtensionTools: tools}, tt.spec)
			for _, g := range tt.enabled {
				if !opts.groupEnabled(g) {
					t.Errorf("group %q should be enabled", g)
				}
			}
			for _, g := range tt.off {
				if opts.groupEnabled(g) {
					t.Errorf("group %q should be disabled", g)
				}
			}
		})
	}
}
