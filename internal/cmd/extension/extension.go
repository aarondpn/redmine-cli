// Package extension implements `redmine extension` (alias `ext`) and the
// top-level commands of installed extensions.
package extension

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// Options configures the extension commands.
type Options struct {
	// Version is the running redmine-cli version (for min_cli_version).
	Version string
	// NewManager returns the extension manager. nil uses the default
	// extensions directory; tests inject a fake GitHub and git.
	NewManager func() (*extmgr.Manager, error)
}

// manager returns the Manager with the core command names of cmd's root,
// which extensions may not take.
func (o Options) manager(cmd *cobra.Command) (*extmgr.Manager, error) {
	var (
		m   *extmgr.Manager
		err error
	)
	if o.NewManager != nil {
		m, err = o.NewManager()
	} else {
		m, err = extmgr.NewManager(o.Version)
	}
	if err != nil {
		return nil, err
	}
	if m.CLIVersion == "" {
		m.CLIVersion = o.Version
	}
	if cmd != nil {
		m.CoreNames = CoreCommandNames(cmd.Root())
	}
	return m, nil
}

// NewCmdExtension creates the `extension` command and its subcommands.
func NewCmdExtension(f *cmdutil.Factory, opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extension",
		Aliases: []string{"ext"},
		Short:   "Manage redmine-cli extensions",
		Long: `Install, list, upgrade and remove extensions: additional top-level
commands (redmine <name> ...) from GitHub repositories named
<owner>/redmine-cli-<name>.

An extension describes its commands in redmine-extension.yml; redmine-cli
derives the CLI command, its flags and help, and an MCP tool from it. The
extension itself reads its input as JSON on stdin and calls Redmine through
"$REDMINE_CLI_BIN api", so it never sees your credentials.

Extensions are not verified by redmine-cli and run with your user
permissions, also as MCP tools. Only install extensions you trust.`,
		Example: `  # Find extensions
  redmine extension search

  # Install one from GitHub; it then runs as 'redmine pr ...'
  redmine extension install octo/redmine-cli-pr

  # Start a new extension and link it for development
  redmine extension create hello
  cd redmine-cli-hello && redmine extension install .`,
	}
	cmd.AddCommand(newCmdInstall(f, opts))
	cmd.AddCommand(newCmdList(f, opts))
	cmd.AddCommand(newCmdUpgrade(f, opts))
	cmd.AddCommand(newCmdRemove(f, opts))
	cmd.AddCommand(newCmdExec(f, opts))
	cmd.AddCommand(newCmdSearch(f, opts))
	cmd.AddCommand(newCmdCreate(f, opts))
	cmd.AddCommand(newCmdCheck(f, opts))
	return cmd
}

// CoreCommandNames returns the names and aliases of root's commands that are
// not extensions.
func CoreCommandNames(root *cobra.Command) []string {
	var names []string
	for _, c := range root.Commands() {
		if IsExtensionCommand(c) {
			continue
		}
		names = append(names, c.Name())
		names = append(names, c.Aliases...)
	}
	return names
}

// completeExtensionNames completes the name of an installed extension as the
// first argument.
func completeExtensionNames(opts Options) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		m, err := opts.manager(nil)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		exts, _ := m.List()
		var out []cobra.Completion
		for _, e := range exts {
			out = append(out, cobra.CompletionWithDesc(e.Name, describe(e)))
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// describe is the one-line description of an installed extension, for
// help and completion. It comes from the extension's files, so it is
// neutralized for the terminal.
func describe(e *extmgr.Extension) string {
	if e.Manifest != nil && e.Manifest.Description != "" {
		return output.SanitizeTerminal(e.Manifest.Description)
	}
	if repo := e.Repo(); repo != "" {
		return output.SanitizeTerminal("Extension from " + repo)
	}
	return "Extension " + e.Name
}
