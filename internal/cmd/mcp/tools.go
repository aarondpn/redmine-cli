package mcp

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
)

func newCmdTools(_ *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List the MCP tool groups and the tools they contain",
		Long: "Print the catalog of tools the MCP server can expose, grouped by " +
			"the value used with --enable-groups / --disable-groups. Read tools " +
			"are always available; tools marked (write) only register when " +
			"--enable-writes is passed to `mcp serve`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printToolCatalog(cmd.OutOrStdout())
		},
	}
	return cmd
}

func printToolCatalog(out io.Writer) error {
	byGroup := mcpserver.ToolsByGroup()
	for _, g := range mcpserver.AllGroups() {
		fmt.Fprintf(out, "%s\n", g)
		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		for _, d := range byGroup[g] {
			kind := "read"
			if d.Writes {
				kind = "write"
			}
			fmt.Fprintf(tw, "  %s\t(%s)\t%s\n", d.Name, kind, d.Description)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}
	return nil
}
