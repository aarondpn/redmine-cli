package mcp

import (
	"os"
	"os/signal"
	"syscall"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
)

func newCmdServe(f *cmdutil.Factory) *cobra.Command {
	var (
		enableWrites bool
		name         string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Redmine MCP server on stdio",
		Long: "Start a Model Context Protocol server on stdio that exposes the " +
			"Redmine API as a set of MCP tools and resources. The active " +
			"--profile (or REDMINE_* environment variables / --server / --api-key " +
			"flags) selects the Redmine instance.\n\n" +
			"By default only read tools are registered. Pass --enable-writes to " +
			"also register create/update/delete tools.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			version := cmd.Root().Version
			if version == "" {
				version = "dev"
			}

			srv := mcpserver.BuildServer(client, mcpserver.Options{
				EnableWrites: enableWrites,
				Name:         name,
				Version:      version,
			})

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return srv.Run(ctx, &sdk.StdioTransport{})
		},
	}

	cmd.Flags().BoolVar(&enableWrites, "enable-writes", false, "Register tools that create, update, or delete Redmine data")
	cmd.Flags().StringVar(&name, "name", "redmine-cli", "Server name advertised to MCP clients")

	return cmd
}
