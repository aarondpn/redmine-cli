package extension

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// trustPrompt is asked before a third-party extension is installed.
const trustPrompt = "Extensions are not verified by redmine-cli and run with your user permissions (also as MCP tools). Install %s?"

func newCmdInstall(f *cmdutil.Factory, opts Options) *cobra.Command {
	var (
		pin   string
		force bool
		yes   bool
	)
	cmd := &cobra.Command{
		Use:   "install <owner/repo | URL | .>",
		Short: "Install an extension from GitHub or a local directory",
		Long: `Install an extension from the GitHub repository <owner>/redmine-cli-<name>,
which adds the command 'redmine <name>'.

If the latest release (or the one given with --pin) has a binary for this
platform (redmine-cli-<name>-<os>-<arch>), it is downloaded and verified
against the release's checksums file when there is one. Otherwise the
repository is cloned with git and its executable redmine-cli-<name> is run.

"." installs the current directory, which must be named redmine-cli-<name>,
as a local extension: it is linked, not copied, so changes take effect
immediately.

Extensions are not verified by redmine-cli and run with your user
permissions, also as MCP tools, so installing one from GitHub asks for
confirmation (--yes skips it; without a terminal it is required).`,
		Example: `  redmine extension install octo/redmine-cli-pr
  redmine extension install https://github.com/octo/redmine-cli-pr --pin v1.2.0
  redmine extension install octo/redmine-cli-pr --yes --force
  redmine extension install .`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := extmgr.ParseSource(args[0])
			if err != nil {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: err.Error(), Err: err}
			}
			m, err := opts.manager(cmd)
			if err != nil {
				return err
			}
			if err := m.ValidateName(src.Name); err != nil {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: err.Error(), Err: err}
			}
			printer := f.Printer("")
			if src.LocalDir == "" && !yes {
				ok, err := confirmTrust(f, printer, src)
				if err != nil || !ok {
					return err
				}
			}

			stop := printer.Spinner("Installing " + src.String() + "...")
			ext, err := m.Install(cmdutil.Context(cmd), src, extmgr.InstallOptions{Pin: pin, Force: force})
			stop()
			if err != nil {
				return installError(err)
			}
			printInstalled(printer, ext)
			return nil
		},
	}
	cmd.Flags().StringVar(&pin, "pin", "", "Install this release or git tag and exclude the extension from upgrades")
	cmd.Flags().BoolVar(&force, "force", false, "Replace an installed extension of the same name")
	cmd.Flags().BoolVar(&yes, "yes", false, "Trust the extension without asking")
	return cmd
}

// confirmTrust shows the trust prompt. Without a terminal it refuses, since
// an unattended install must say --yes explicitly.
func confirmTrust(f *cmdutil.Factory, printer output.Printer, src extmgr.Source) (bool, error) {
	if !stdinIsTerminal(f) {
		return false, &cmdutil.CodedError{
			Code:    output.ErrCodeValidationFailed,
			Message: fmt.Sprintf("installing %s needs confirmation: pass --yes to trust it (there is no terminal to ask on)", src),
		}
	}
	if cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, fmt.Sprintf(trustPrompt, src)) {
		return true, nil
	}
	printer.Outcome(false, output.ActionInstalled, "extension", src.Name, "Install cancelled")
	return false, nil
}

// installError gives install failures a stable JSON error code.
func installError(err error) error {
	var (
		already *extmgr.AlreadyInstalledError
		invalid *extmgr.InvalidManifestError
		ve      *spec.ValidationError
	)
	switch {
	case errors.As(err, &already):
		return &cmdutil.CodedError{Code: output.ErrCodeAlreadyExists, Message: err.Error(), Err: err}
	case errors.As(err, &invalid) && errors.As(err, &ve):
		return &cmdutil.CodedError{
			Code:    output.ErrCodeValidationFailed,
			Message: fmt.Sprintf("%s has an invalid %s", invalid.Source, spec.FileName),
			Details: ve.Details(),
			Err:     err,
		}
	}
	return err
}

// extensionSummary is the JSON shape of an installed extension.
type extensionSummary struct {
	Name     string   `json:"name"`
	Repo     string   `json:"repo,omitempty"`
	Version  string   `json:"version,omitempty"`
	Kind     string   `json:"kind"`
	Pinned   bool     `json:"pinned"`
	Path     string   `json:"path"`
	Commands []string `json:"commands"`
	MCPTools []string `json:"mcp_tools"`
}

func summarize(ext *extmgr.Extension) extensionSummary {
	return extensionSummary{
		Name:     ext.Name,
		Repo:     ext.Repo(),
		Version:  ext.Version(),
		Kind:     string(ext.Kind),
		Pinned:   ext.Pinned(),
		Path:     ext.Path,
		Commands: cliCommands(ext),
		MCPTools: mcpTools(ext),
	}
}

// cliCommands lists the commands an extension adds, as usage lines.
func cliCommands(ext *extmgr.Extension) []string {
	if ext.Manifest == nil {
		return []string{"redmine " + ext.Name + " [args...]"}
	}
	out := make([]string, 0, len(ext.Manifest.Commands))
	for i := range ext.Manifest.Commands {
		c := &ext.Manifest.Commands[i]
		line := "redmine " + ext.Name + " " + spec.UseLine(c)
		if c.Raw {
			line = "redmine " + ext.Name + " " + c.Name + " [args...]"
		}
		out = append(out, line)
	}
	return out
}

// mcpTools lists the MCP tools an extension adds.
func mcpTools(ext *extmgr.Extension) []string {
	out := []string{}
	if ext.Manifest == nil {
		return out
	}
	for _, c := range ext.Manifest.MCPCommands() {
		out = append(out, spec.ToolName(ext.Name, c.Name))
	}
	return out
}

func printInstalled(printer output.Printer, ext *extmgr.Extension) {
	s := summarize(ext)
	if printer.Format() == output.FormatJSON {
		printer.JSON(s)
		return
	}
	tools := strings.Join(s.MCPTools, ", ")
	if tools == "" {
		tools = "none"
	}
	pairs := []output.KeyValue{{Key: "Name", Value: s.Name}}
	if s.Repo != "" {
		pairs = append(pairs, output.KeyValue{Key: "Repo", Value: s.Repo})
	}
	if s.Version != "" {
		pairs = append(pairs, output.KeyValue{Key: "Version", Value: s.Version})
	}
	pairs = append(pairs,
		output.KeyValue{Key: "Kind", Value: s.Kind},
		output.KeyValue{Key: "Commands", Value: strings.Join(s.Commands, ", ")},
		output.KeyValue{Key: "MCP tools", Value: tools},
	)
	printer.Detail(pairs)
	printer.Success(fmt.Sprintf("Installed extension %s; run 'redmine %s --help' to get started", s.Name, s.Name))
}
