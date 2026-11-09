package mcp

import (
	"fmt"
	"io"
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
			"stdio by default and over --http only with --enable-extensions.",
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

func printToolCatalog(out io.Writer, catalog *mcpserver.Catalog) error {
	byGroup := catalog.ToolsByGroup()
	groups := append(mcpserver.AllGroups(), catalog.ExtensionGroups()...)
	for _, g := range groups {
		fmt.Fprintf(out, "%s\n", g)
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		for _, d := range byGroup[g] {
			// Extension descriptions come from a manifest on disk.
			fmt.Fprintf(tw, "  %s\t(%s)\t%s\n", d.Name, output.SanitizeTerminal(toolOrigin(d)), output.SanitizeTerminal(d.Description))
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
