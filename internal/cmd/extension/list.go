package extension

import (
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// listItem is the JSON shape of `extension list`.
type listItem struct {
	Name     string `json:"name"`
	Repo     string `json:"repo,omitempty"`
	Version  string `json:"version,omitempty"`
	Kind     string `json:"kind"`
	Pinned   bool   `json:"pinned"`
	Commands int    `json:"commands"`
	MCPTools int    `json:"mcp_tools"`
	Path     string `json:"path"`
	// Shadowed: a core command has the same name, so the extension only
	// runs through `redmine extension exec`.
	Shadowed bool   `json:"shadowed"`
	Error    string `json:"error,omitempty"`
}

func newCmdList(f *cmdutil.Factory, opts Options) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List installed extensions",
		Example: `  redmine extension list
  redmine extension list --jq '.[] | select(.error) | .name'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := opts.manager(cmd)
			if err != nil {
				return err
			}
			exts, err := m.List()
			if err != nil {
				return err
			}
			items := make([]listItem, 0, len(exts))
			for _, e := range exts {
				items = append(items, toListItem(e, m.CoreNames))
			}
			printer := f.Printer("")
			if cmdutil.HandleEmpty(printer, items, "extensions") {
				return nil
			}
			if printer.Format() == output.FormatJSON {
				printer.JSON(items)
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				rows = append(rows, []string{
					it.Name, it.Repo, it.Version, it.Kind, yesNo(it.Pinned),
					strconv.Itoa(it.Commands), strconv.Itoa(it.MCPTools), notes(it),
				})
			}
			printer.Table([]string{"Name", "Repo", "Version", "Kind", "Pinned", "Commands", "MCP Tools", "Notes"}, rows)
			return nil
		},
	}
}

func toListItem(e *extmgr.Extension, core []string) listItem {
	it := listItem{
		Name:     e.Name,
		Repo:     e.Repo(),
		Version:  e.Version(),
		Kind:     string(e.Kind),
		Pinned:   e.Pinned(),
		Path:     e.Path,
		Shadowed: slices.Contains(core, e.Name),
	}
	if e.Err != nil {
		it.Error = e.Err.Error()
	}
	if e.Manifest != nil {
		it.Commands = len(e.Manifest.Commands)
		it.MCPTools = len(e.Manifest.MCPCommands())
	}
	return it
}

func notes(it listItem) string {
	var out []string
	if it.Shadowed {
		out = append(out, "shadowed by a core command; use 'redmine extension exec "+it.Name+"'")
	}
	if it.Error != "" {
		msg, _, _ := strings.Cut(it.Error, "\n")
		out = append(out, "broken: "+msg)
	} else if it.Commands == 0 {
		out = append(out, "no manifest (passthrough)")
	}
	return strings.Join(out, "; ")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
