package mcpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func listResourceTemplateURIs(t *testing.T, cs *mcp.ClientSession) map[string]bool {
	t.Helper()
	res, err := cs.ListResourceTemplates(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}
	templates := make(map[string]bool, len(res.ResourceTemplates))
	for _, rt := range res.ResourceTemplates {
		templates[rt.URITemplate] = true
	}
	return templates
}

// TestResourceTemplatesFollowToolFilter pins M-03: --enable-tools and
// --disable-tools also decide which resource templates are advertised, so a
// resource cannot read what the per-tool filter hides.
func TestResourceTemplatesFollowToolFilter(t *testing.T) {
	all := []string{tmplIssue, tmplProject, tmplUser, tmplTimeEntry, tmplWiki, tmplVersion}
	tests := []struct {
		name string
		spec FilterSpec
		want []string
	}{
		{
			name: "no filter keeps every resource",
			spec: FilterSpec{},
			want: all,
		},
		{
			name: "allow-list without get tools hides every resource",
			spec: FilterSpec{EnableTools: []string{"list_issues"}},
			want: nil,
		},
		{
			name: "allow-list exposes only the matching resource",
			spec: FilterSpec{EnableTools: []string{"list_issues", "get_issue"}},
			want: []string{tmplIssue},
		},
		{
			name: "deny get_issue hides the issue resource",
			spec: FilterSpec{DisableTools: []string{"get_issue"}},
			want: []string{tmplProject, tmplUser, tmplTimeEntry, tmplWiki, tmplVersion},
		},
		{
			name: "deny both user tools hides the user resource",
			spec: FilterSpec{DisableTools: []string{"get_user", "me"}},
			want: []string{tmplIssue, tmplProject, tmplTimeEntry, tmplWiki, tmplVersion},
		},
		{
			name: "either user tool keeps the user resource",
			spec: FilterSpec{EnableTools: []string{"me"}},
			want: []string{tmplUser},
		},
		{
			name: "each mapped tool enables its resource",
			spec: FilterSpec{EnableTools: []string{"get_project", "get_time_entry", "get_wiki_page", "get_version", "get_user"}},
			want: []string{tmplProject, tmplUser, tmplTimeEntry, tmplWiki, tmplVersion},
		},
		{
			name: "group filter still applies on top of the tool filter",
			spec: FilterSpec{EnableGroups: []Group{GroupIssues}, EnableTools: []string{"get_issue", "get_project"}},
			want: []string{tmplIssue},
		},
	}
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, cleanup := newConnectedSession(t, apiClient, ResolveOptions(Options{Version: "v0"}, tt.spec))
			defer cleanup()

			got := listResourceTemplateURIs(t, cs)
			want := make(map[string]bool, len(tt.want))
			for _, u := range tt.want {
				want[u] = true
			}
			for _, u := range all {
				if got[u] != want[u] {
					t.Errorf("template %q registered=%v, want %v", u, got[u], want[u])
				}
			}
		})
	}
}

// TestUserResourceRejectsFilteredHalf covers the user resource, which is
// backed by two tools: redmine://user/me follows "me" and numeric IDs follow
// get_user.
func TestUserResourceRejectsFilteredHalf(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/current.json", "/users/1.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user":{"id":1,"login":"alice"}}`))
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer closeTS()

	tests := []struct {
		name    string
		disable string
		allowed string
		denied  string
	}{
		{"me disabled", "me", "redmine://user/1", "redmine://user/me"},
		{"get_user disabled", "get_user", "redmine://user/me", "redmine://user/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ResolveOptions(Options{Version: "v0"}, FilterSpec{DisableTools: []string{tt.disable}})
			cs, cleanup := newConnectedSession(t, apiClient, opts)
			defer cleanup()
			ctx := context.Background()

			res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: tt.allowed})
			if err != nil {
				t.Fatalf("ReadResource(%s): %v", tt.allowed, err)
			}
			if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "alice") {
				t.Errorf("unexpected contents for %s: %+v", tt.allowed, res.Contents)
			}

			_, err = cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: tt.denied})
			if err == nil {
				t.Fatalf("ReadResource(%s) should fail while %q is disabled", tt.denied, tt.disable)
			}
			if !strings.Contains(err.Error(), tt.disable) {
				t.Errorf("error should name the disabled tool %q: %v", tt.disable, err)
			}
		})
	}
}

// TestResourceDefinitionsMapToKnownTools keeps every resource tied to at least
// one real read tool in its own group, so a new resource cannot silently
// bypass the per-tool filter.
func TestResourceDefinitionsMapToKnownTools(t *testing.T) {
	byName := make(map[string]ToolDescriptor)
	for _, d := range AllTools() {
		byName[d.Name] = d
	}
	for _, def := range resourceTemplateDefinitions(Options{}) {
		if len(def.Tools) == 0 {
			t.Errorf("resource %q is not mapped to any tool", def.Template.URITemplate)
		}
		for _, name := range def.Tools {
			d, ok := byName[name]
			if !ok {
				t.Errorf("resource %q maps to unknown tool %q", def.Template.URITemplate, name)
				continue
			}
			if d.Writes {
				t.Errorf("resource %q maps to write tool %q", def.Template.URITemplate, name)
			}
			if d.Group != def.Group {
				t.Errorf("resource %q (group %q) maps to tool %q in group %q", def.Template.URITemplate, def.Group, name, d.Group)
			}
		}
	}
}

func TestOptionsAnyToolEnabled(t *testing.T) {
	opts := Options{
		EnabledTools:  map[string]bool{"me": true, "get_user": true},
		DisabledTools: map[string]bool{"get_user": true},
	}
	if !opts.anyToolEnabled([]string{"get_user", "me"}) {
		t.Error("me is allowed, so the pair should be enabled")
	}
	if opts.anyToolEnabled([]string{"get_user"}) {
		t.Error("get_user is denied")
	}
	if opts.anyToolEnabled([]string{"get_issue"}) {
		t.Error("get_issue is outside the allow-list")
	}
	if !(Options{}).anyToolEnabled([]string{"get_issue"}) {
		t.Error("no filter should enable every tool")
	}
}
