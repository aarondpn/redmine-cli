package mcp

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdTools(f *cmdutil.Factory, opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List the MCP tool groups and the tools they contain",
		Long: "Print the catalog of tools the MCP server can expose, grouped by " +
			"the value used with --enable-groups / --disable-groups. Read tools " +
			"are always available; tools marked (write) only register when " +
			"--enable-writes is passed to `mcp serve`. Tools of installed " +
			"extensions are listed in one ext-<name> group per extension, with " +
			"the extension and its version; `mcp serve` registers them on " +
			"stdio by default and over --http only with --enable-extensions. " +
			"An extension tool whose name is already taken (by a core tool, or " +
			"by the extension that comes first by name) is not served; it is " +
			"listed in its group marked skipped, with the tool it clashes with.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exts, err := installedExtensions(opts)
			if err != nil {
				f.Warn(fmt.Sprintf("could not list the installed extensions: %v", err))
			}
			for _, ext := range exts {
				if ext.Err != nil {
					f.Warn(fmt.Sprintf("extension %s is unavailable and provides no MCP tools: %v", ext.Name, ext.Err))
				}
			}
			tools := extensionTools(exts, runner.Env{})
			_, clashes := mcpserver.ResolveExtensionTools(tools)
			for _, w := range clashes {
				f.Warn(w)
			}
			return printToolCatalog(cmd.OutOrStdout(), mcpserver.NewCatalog(tools))
		},
	}
	cmdutil.MarkNoJSONOutput(cmd)
	return cmd
}

// catalogRow is a line of the tool listing.
type catalogRow struct {
	tool mcpserver.ToolDescriptor
	// skipped is the reason the tool is not served, empty for a tool that
	// is.
	skipped string
}

func printToolCatalog(out io.Writer, catalog *mcpserver.Catalog) error {
	byGroup := catalog.ToolsByGroup()
	rows := map[mcpserver.Group][]catalogRow{}
	for g, ds := range byGroup {
		for _, d := range ds {
			rows[g] = append(rows[g], catalogRow{tool: d})
		}
	}
	// A tool skipped for a name clash is listed in its extension's group,
	// marked, so the clash shows here and not only in a warning; that group
	// is listed even when the extension has no other tool.
	extGroups := catalog.ExtensionGroups()
	for _, s := range catalog.SkippedTools() {
		if !slices.Contains(extGroups, s.Group) {
			extGroups = append(extGroups, s.Group)
		}
		rows[s.Group] = append(rows[s.Group], catalogRow{tool: s.ToolDescriptor, skipped: s.Reason()})
	}
	slices.Sort(extGroups)
	for _, g := range extGroups {
		slices.SortStableFunc(rows[g], func(a, b catalogRow) int { return strings.Compare(a.tool.Name, b.tool.Name) })
	}

	groups := append(mcpserver.AllGroups(), extGroups...)
	for _, g := range groups {
		fmt.Fprintf(out, "%s\n", g)
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		for _, r := range rows[g] {
			origin := toolOrigin(r.tool)
			if r.skipped != "" {
				origin += ", skipped: " + r.skipped
			}
			// Extension descriptions come from a manifest on disk.
			fmt.Fprintf(tw, "  %s\t(%s)\t%s\n", r.tool.Name, output.SanitizeTerminal(origin), output.SanitizeTerminal(r.tool.Description))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}
	if len(catalog.ExtensionGroups()) > 0 {
		fmt.Fprintf(out, "%s\n  Selects every ext-<name> group above.\n\n", mcpserver.GroupExtensions)
	}
	return nil
}

// toolOrigin is the parenthesized note after a tool name: read or write,
// and for an extension tool the extension and its version, e.g.
// "read, extension pr@v1.2.0".
func toolOrigin(d mcpserver.ToolDescriptor) string {
	kind := "read"
	if d.Writes {
		kind = "write"
	}
	if d.Extension == "" {
		return kind
	}
	origin := d.Extension
	if d.ExtensionVersion != "" {
		origin += "@" + d.ExtensionVersion
	}
	return kind + ", extension " + origin
}
