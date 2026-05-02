package mcp

import (
	"errors"
	"net/http"
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
		httpAddr     string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Redmine MCP server on stdio or HTTP",
		Long: "Start a Model Context Protocol server on stdio or streamable HTTP that exposes the " +
			"Redmine API as a set of MCP tools and resources. The active " +
			"--profile (or REDMINE_* environment variables / --server / --api-key " +
			"flags) selects the Redmine instance.\n\n" +
			"By default only read tools are registered. Pass --enable-writes to " +
			"also register create/update/delete tools. Use --http to listen on " +
			"an HTTP address such as :8080 instead of stdio.",
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

			opts := mcpserver.Options{
				EnableWrites: enableWrites,
				Name:         name,
				Version:      version,
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if httpAddr != "" {
				handler := mcpserver.BuildHTTPHandler(client, opts)
				server := &http.Server{
					Addr:    httpAddr,
					Handler: handler,
				}
				go func() {
					<-ctx.Done()
					_ = server.Close()
				}()
				err = server.ListenAndServe()
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			}

			srv := mcpserver.BuildServer(client, opts)
			return srv.Run(ctx, &sdk.StdioTransport{})
		},
	}

	cmd.Flags().BoolVar(&enableWrites, "enable-writes", false, "Register tools that create, update, or delete Redmine data")
	cmd.Flags().StringVar(&httpAddr, "http", "", "Serve MCP over streamable HTTP on the given address instead of stdio (for example :8080)")
	cmd.Flags().StringVar(&name, "name", "redmine-cli", "Server name advertised to MCP clients")

	return cmd
}
