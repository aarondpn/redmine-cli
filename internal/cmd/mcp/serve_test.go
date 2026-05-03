package mcp

import (
	"bytes"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
)

func TestNewCmdMCP_HasServe(t *testing.T) {
	cmd := NewCmdMCP(cmdutil.NewFactory())
	if cmd.Use != "mcp" {
		t.Errorf("Use = %q, want mcp", cmd.Use)
	}
	subcommands := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subcommands[sub.Use] = true
	}
	for _, want := range []string{"serve", "tools"} {
		if !subcommands[want] {
			t.Errorf("mcp %s subcommand not registered", want)
		}
	}
}

func TestServeFlags_DefaultsToReadOnly(t *testing.T) {
	cmd := newCmdServe(cmdutil.NewFactory())

	enable, err := cmd.Flags().GetBool("enable-writes")
	if err != nil {
		t.Fatalf("GetBool: %v", err)
	}
	if enable {
		t.Error("--enable-writes defaulted to true; expected false")
	}

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if name != "redmine-cli" {
		t.Errorf("--name default = %q, want redmine-cli", name)
	}

	httpAddr, err := cmd.Flags().GetString("http")
	if err != nil {
		t.Fatalf("GetString(http): %v", err)
	}
	if httpAddr != "" {
		t.Errorf("--http default = %q, want empty string", httpAddr)
	}
}

func TestBuildFilterSpec_FlagsBeatConfig(t *testing.T) {
	cfg := &config.Config{MCP: config.MCPConfig{
		EnableGroups: []string{"issues"},
		DisableTools: []string{"delete_issue"},
	}}

	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("enable-groups", nil, "")
	cmd.Flags().StringSlice("disable-groups", nil, "")
	cmd.Flags().StringSlice("enable-tools", nil, "")
	cmd.Flags().StringSlice("disable-tools", nil, "")

	if err := cmd.Flags().Set("enable-groups", "wiki,projects"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	enableGroups, _ := cmd.Flags().GetStringSlice("enable-groups")
	disableGroups, _ := cmd.Flags().GetStringSlice("disable-groups")
	enableTools, _ := cmd.Flags().GetStringSlice("enable-tools")
	disableTools, _ := cmd.Flags().GetStringSlice("disable-tools")

	spec, err := buildFilterSpec(cfg, cmd, enableGroups, disableGroups, enableTools, disableTools)
	if err != nil {
		t.Fatalf("buildFilterSpec: %v", err)
	}

	gotGroups := make([]string, 0, len(spec.EnableGroups))
	for _, g := range spec.EnableGroups {
		gotGroups = append(gotGroups, string(g))
	}
	sort.Strings(gotGroups)
	wantGroups := []string{"projects", "wiki"}
	if !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Errorf("EnableGroups from flag = %v, want %v (config should be ignored)", gotGroups, wantGroups)
	}

	if !reflect.DeepEqual(spec.DisableTools, []string{"delete_issue"}) {
		t.Errorf("DisableTools should fall back to config when flag unset, got %v", spec.DisableTools)
	}
}

func TestBuildFilterSpec_ConfigUsedWhenFlagsAbsent(t *testing.T) {
	cfg := &config.Config{MCP: config.MCPConfig{
		EnableGroups: []string{"issues", "wiki"},
		EnableTools:  []string{"list_issues"},
	}}

	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("enable-groups", nil, "")
	cmd.Flags().StringSlice("disable-groups", nil, "")
	cmd.Flags().StringSlice("enable-tools", nil, "")
	cmd.Flags().StringSlice("disable-tools", nil, "")

	spec, err := buildFilterSpec(cfg, cmd, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildFilterSpec: %v", err)
	}

	got := make([]mcpserver.Group, 0, len(spec.EnableGroups))
	got = append(got, spec.EnableGroups...)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	want := []mcpserver.Group{mcpserver.GroupIssues, mcpserver.GroupWiki}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EnableGroups = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(spec.EnableTools, []string{"list_issues"}) {
		t.Errorf("EnableTools = %v, want [list_issues]", spec.EnableTools)
	}
}

func TestResolveEnableWrites(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("enable-writes", false, "")

	tr := true
	cfg := &config.Config{MCP: config.MCPConfig{EnableWrites: &tr}}

	if !resolveEnableWrites(cmd, false, cfg) {
		t.Error("config should win when flag is unset")
	}

	if err := cmd.Flags().Set("enable-writes", "false"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if resolveEnableWrites(cmd, false, cfg) {
		t.Error("explicit flag should override config")
	}
}

func TestResolveAuthToken(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("auth-token", "", "")

	cfg := &config.Config{MCP: config.MCPConfig{AuthToken: "from-config"}}

	if got := resolveAuthToken(cmd, "", cfg); got != "from-config" {
		t.Errorf("config token should win when flag is unset, got %q", got)
	}

	if err := cmd.Flags().Set("auth-token", "from-flag"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := resolveAuthToken(cmd, "from-flag", cfg); got != "from-flag" {
		t.Errorf("explicit flag should override config, got %q", got)
	}

	cmd2 := &cobra.Command{}
	cmd2.Flags().String("auth-token", "", "")
	if err := cmd2.Flags().Set("auth-token", ""); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := resolveAuthToken(cmd2, "", cfg); got != "" {
		t.Errorf("explicitly empty flag should suppress config, got %q", got)
	}
}

func TestBuildFilterSpec_RejectsUnknownTool(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("enable-groups", nil, "")
	cmd.Flags().StringSlice("disable-groups", nil, "")
	cmd.Flags().StringSlice("enable-tools", nil, "")
	cmd.Flags().StringSlice("disable-tools", nil, "")

	if err := cmd.Flags().Set("enable-tools", "list_issues,not_a_real_tool"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	enableGroups, _ := cmd.Flags().GetStringSlice("enable-groups")
	disableGroups, _ := cmd.Flags().GetStringSlice("disable-groups")
	enableTools, _ := cmd.Flags().GetStringSlice("enable-tools")
	disableTools, _ := cmd.Flags().GetStringSlice("disable-tools")

	_, err := buildFilterSpec(nil, cmd, enableGroups, disableGroups, enableTools, disableTools)
	if err == nil {
		t.Fatal("buildFilterSpec should reject unknown tool names")
	}
	if !strings.Contains(err.Error(), `unknown tool "not_a_real_tool"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServeHelp_EnableGroupsUsageRendersNormally(t *testing.T) {
	cmd := newCmdServe(cmdutil.NewFactory())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Help(); err != nil {
		t.Fatalf("Help: %v", err)
	}

	help := out.String()
	if strings.Contains(help, "--enable-groups redmine mcp tools") {
		t.Fatalf("help output used command text as value placeholder:\n%s", help)
	}
	if !strings.Contains(help, "--enable-groups strings") {
		t.Fatalf("help output missing normal StringSlice placeholder:\n%s", help)
	}
}
