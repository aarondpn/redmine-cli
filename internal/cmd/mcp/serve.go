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
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
)

func newCmdServe(f *cmdutil.Factory) *cobra.Command {
	var (
		enableWrites  bool
		name          string
		httpAddr      string
		enableGroups  []string
		disableGroups []string
		enableTools   []string
		disableTools  []string
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
			"an HTTP address such as :8080 instead of stdio.\n\n" +
			"To narrow the set of tools exposed to MCP clients, use " +
			"--enable-groups / --disable-groups (allow- and deny-list of tool " +
			"groups) and --enable-tools / --disable-tools (per-tool overrides). " +
			"Run `redmine mcp tools` to see the available groups and tools.",
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

			cfg, _ := f.Config()
			spec, err := buildFilterSpec(cfg, cmd, enableGroups, disableGroups, enableTools, disableTools)
			if err != nil {
				return err
			}

			opts := mcpserver.Options{
				EnableWrites: resolveEnableWrites(cmd, enableWrites, cfg),
				Name:         name,
				Version:      version,
			}
			opts = mcpserver.ResolveOptions(opts, spec)

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
	cmd.Flags().StringSliceVar(&enableGroups, "enable-groups", nil, "Comma-separated tool groups to expose (default: all). See redmine mcp tools.")
	cmd.Flags().StringSliceVar(&disableGroups, "disable-groups", nil, "Comma-separated tool groups to hide. Applied after --enable-groups.")
	cmd.Flags().StringSliceVar(&enableTools, "enable-tools", nil, "Allow-list of tool names. Tools outside the list are hidden.")
	cmd.Flags().StringSliceVar(&disableTools, "disable-tools", nil, "Deny-list of tool names. Applied after --enable-tools.")

	return cmd
}

// buildFilterSpec merges config-file values with the CLI flag values. CLI
// flags take precedence: when a flag is explicitly set, the corresponding
// config value is ignored entirely (rather than appended).
func buildFilterSpec(cfg *config.Config, cmd *cobra.Command, enableGroupsFlag, disableGroupsFlag, enableToolsFlag, disableToolsFlag []string) (mcpserver.FilterSpec, error) {
	pick := func(flagName string, fromFlag, fromCfg []string) []string {
		if cmd.Flags().Changed(flagName) {
			return fromFlag
		}
		return fromCfg
	}

	var cfgEG, cfgDG, cfgET, cfgDT []string
	if cfg != nil {
		cfgEG = cfg.MCP.EnableGroups
		cfgDG = cfg.MCP.DisableGroups
		cfgET = cfg.MCP.EnableTools
		cfgDT = cfg.MCP.DisableTools
	}

	eg := pick("enable-groups", enableGroupsFlag, cfgEG)
	dg := pick("disable-groups", disableGroupsFlag, cfgDG)
	et := pick("enable-tools", enableToolsFlag, cfgET)
	dt := pick("disable-tools", disableToolsFlag, cfgDT)

	parsedEnable, err := mcpserver.ParseGroups(eg)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}
	parsedDisable, err := mcpserver.ParseGroups(dg)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}
	parsedEnableTools, err := mcpserver.ParseTools(et)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}
	parsedDisableTools, err := mcpserver.ParseTools(dt)
	if err != nil {
		return mcpserver.FilterSpec{}, err
	}

	return mcpserver.FilterSpec{
		EnableGroups:  parsedEnable,
		DisableGroups: parsedDisable,
		EnableTools:   parsedEnableTools,
		DisableTools:  parsedDisableTools,
	}, nil
}

// resolveEnableWrites returns the effective --enable-writes setting. Flag
// takes precedence; otherwise the config-file MCP block wins; otherwise the
// flag default.
func resolveEnableWrites(cmd *cobra.Command, flagVal bool, cfg *config.Config) bool {
	if cmd.Flags().Changed("enable-writes") {
		return flagVal
	}
	if cfg != nil && cfg.MCP.EnableWrites != nil {
		return *cfg.MCP.EnableWrites
	}
	return flagVal
}
