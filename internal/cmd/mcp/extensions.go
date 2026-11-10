package mcp

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
)

// enableExtensionsFlag is the `mcp serve` flag that turns extension tools
// on or off.
const enableExtensionsFlag = "enable-extensions"

// apiKeyNotForwardedWarning mirrors the CLI's warning: an extension's own
// redmine calls never see --api-key, they use the profile's credentials.
const apiKeyNotForwardedWarning = "--api-key and --api-key-file are not passed to extensions; the redmine calls an extension tool makes use the profile's credentials (or REDMINE_API_KEY)"

// installedExtensions lists the installed extensions. A listing that fails
// (e.g. an unreadable extensions directory) yields none, with the error, so
// the caller can decide whether it is worth a warning: the server must
// start either way.
func installedExtensions(opts Options) ([]*extmgr.Extension, error) {
	if opts.Extensions == nil {
		return nil, nil
	}
	return opts.Extensions()
}

// extensionTools returns one tool per MCP command of every usable
// extension, with env as the invocation-level environment of each. Broken
// extensions and extensions without a manifest contribute nothing.
func extensionTools(exts []*extmgr.Extension, env runner.Env) []mcpserver.ExtensionTool {
	var tools []mcpserver.ExtensionTool
	for _, ext := range exts {
		if ext.Err != nil || ext.Manifest == nil {
			continue
		}
		for _, c := range ext.Manifest.MCPCommands() {
			tools = append(tools, mcpserver.ExtensionTool{
				Extension: ext.Name,
				Version:   ext.Version(),
				Path:      ext.Path,
				Command:   c,
				Env:       env,
			})
		}
	}
	return tools
}

// extensionCatalog is the catalog filter input is validated against. It
// holds the tools of every installed extension, whether or not this run
// serves them, so a configuration naming an extension's group or tool keeps
// working on a transport where extensions are off.
func extensionCatalog(exts []*extmgr.Extension) *mcpserver.Catalog {
	return mcpserver.NewCatalog(extensionTools(exts, runner.Env{}))
}

// resolveEnableExtensions returns whether extension tools are served.
// Precedence: --enable-extensions, then mcp.enable_extensions (which
// REDMINE_MCP_ENABLE_EXTENSIONS overrides while the config loads), then the
// transport default: on for stdio, where the host started the server for
// the local user, and off for HTTP, where any client that reaches the port
// would run the user's extensions.
func resolveEnableExtensions(cmd *cobra.Command, flagVal bool, cfg *config.Config, httpMode bool) bool {
	if cmd.Flags().Changed(enableExtensionsFlag) {
		return flagVal
	}
	if cfg != nil && cfg.MCP.EnableExtensions != nil {
		return *cfg.MCP.EnableExtensions
	}
	return !httpMode
}

// serveExtensionTools builds the extension tools a server registers. Every
// tool gets the environment contract of this invocation, built the same way
// as for a CLI run (profile, config file, server and headers of the MCP
// process). Problems that cost tools but not the server (an unreadable
// extensions directory, a broken extension, a name clash) are reported as
// warnings on stderr.
func serveExtensionTools(f *cmdutil.Factory, exts []*extmgr.Extension, listErr error) ([]mcpserver.ExtensionTool, error) {
	if listErr != nil {
		f.Warn(fmt.Sprintf("could not list the installed extensions, serving without extension tools: %v", listErr))
		return nil, nil
	}
	for _, ext := range exts {
		if ext.Err != nil {
			f.Warn(fmt.Sprintf("extension %s is unavailable and provides no MCP tools: %v", ext.Name, ext.Err))
		}
	}
	tools := extensionTools(exts, runner.Env{})
	if len(tools) == 0 {
		return nil, nil
	}
	env, err := runner.FromFactory(f)
	if err != nil {
		return nil, err
	}
	for i := range tools {
		tools[i].Env = env
	}
	kept, warnings := mcpserver.ResolveExtensionTools(tools)
	for _, w := range warnings {
		f.Warn(w)
	}
	if len(kept) > 0 && (f.APIKeyOverride != "" || f.APIKeyFile != "") {
		f.Warn(apiKeyNotForwardedWarning)
	}
	return kept, nil
}
