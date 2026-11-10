package extension

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// checkedCommand is the JSON shape of one command in `extension check`.
type checkedCommand struct {
	Name        string `json:"name"`
	Usage       string `json:"usage"`
	MCPTool     string `json:"mcp_tool,omitempty"`
	Writes      bool   `json:"writes"`
	Destructive bool   `json:"destructive"`
	Raw         bool   `json:"raw"`
}

type checkResult struct {
	Name     string           `json:"name"`
	File     string           `json:"file"`
	Valid    bool             `json:"valid"`
	Commands []checkedCommand `json:"commands"`
}

func newCmdCheck(f *cmdutil.Factory, opts Options) *cobra.Command {
	var (
		command string
		input   string
		mcp     bool
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "check [<name> | <dir>]",
		Short: "Validate an extension manifest and try a command",
		Long: `Validate redmine-extension.yml of an installed extension (by name) or of a
directory (default: the current one) against the JSON Schema and the
semantic rules, and list the CLI commands and MCP tools it defines.

With --command, run one declarative command through the same protocol the
CLI and the MCP server use: --input is validated like MCP tool arguments
(JSON object; @file reads a file, @- stdin), the input is sent on stdin and
the result is printed like the CLI command would print it. --mcp sets the
environment of the MCP surface (REDMINE_EXT_SURFACE=mcp, REDMINE_MCP=1).
A destructive command asks for confirmation, as it does on the CLI, unless
--yes is given.`,
		Example: `  redmine extension check
  redmine extension check pr
  redmine extension check . --command list --input '{"issue_id": 123}'
  redmine extension check . --command list --input @input.json --mcp -o json`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeExtensionNames(opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			if command == "" && (cmd.Flags().Changed("input") || mcp || yes) {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: "--input, --mcp and --yes require --command"}
			}
			ext, err := checkTarget(cmd, opts, target)
			if err != nil {
				return err
			}
			path := filepath.Join(ext.Dir, spec.FileName)
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				return &cmdutil.CodedError{Code: output.ErrCodeNotFound, Message: fmt.Sprintf("no %s in %s; an extension without a manifest is passed its arguments unchanged", spec.FileName, ext.Dir)}
			}
			man, err := spec.ParseFile(path, spec.Options{ExtensionName: ext.Name, CLIVersion: opts.Version})
			if err != nil {
				var ve *spec.ValidationError
				if errors.As(err, &ve) {
					return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: path + " is invalid", Details: ve.Details(), Err: err}
				}
				return err
			}
			if ext.Name == "" {
				ext.Name = man.Name
				ext.Path = extmgr.ExecutablePath(ext.Dir, man.Name)
			}
			ext.Manifest = man
			// The checks install makes beyond the manifest, so that a
			// manifest check passes does not pass what install rejects.
			rules := &extmgr.Manager{CoreNames: CoreCommandNames(cmd.Root())}
			if err := rules.ValidateName(ext.Name); err != nil {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: err.Error(), Err: err}
			}
			if err := rules.CheckExecutable(ext.Dir, ext.Name); err != nil {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: err.Error(), Err: err}
			}

			if command == "" {
				printCheck(f.Printer(""), ext, path)
				return nil
			}
			c := man.Command(command)
			if c == nil {
				names := make([]string, 0, len(man.Commands))
				for _, mc := range man.Commands {
					names = append(names, mc.Name)
				}
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: fmt.Sprintf("%s has no command %q (commands: %s)", spec.FileName, command, strings.Join(names, ", "))}
			}
			if c.Raw {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: fmt.Sprintf("%q is a raw command and takes no input; run it with 'redmine %s %s'", command, ext.Name, command)}
			}
			surface := runner.SurfaceCLI
			if mcp {
				if !c.MCPEnabled() {
					return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: fmt.Sprintf("%q is not an MCP tool (mcp: false)", command)}
				}
				surface = runner.SurfaceMCP
			}
			raw, err := readInputArg(input, cmd.InOrStdin())
			if err != nil {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: "--input: " + err.Error(), Err: err}
			}
			// Decoded like MCP tool arguments, so an integer beyond 2^53 or
			// a json param keeps its exact digits as it does over MCP.
			obj, err := spec.DecodeArguments(raw)
			if trimmed := bytes.TrimSpace(raw); err != nil || len(trimmed) == 0 || string(trimmed) == "null" {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: "--input must be a JSON object"}
			}
			prepared, err := spec.PrepareInput(c, spec.NormalizeArguments(c, obj))
			if err != nil {
				return inputError(err)
			}
			label := "redmine " + ext.Name + " " + c.Name
			// check runs the command for real: a destructive one needs the
			// same confirmation as on the CLI.
			if c.Destructive && !yes {
				ok, err := confirmDestructive(f, label)
				if err != nil || !ok {
					return err
				}
			}
			return runDeclarativeWith(cmd, f, ext, c, prepared, surface, label, opts, false)
		},
	}
	cmd.Flags().StringVar(&command, "command", "", "Run this declarative command")
	cmd.Flags().StringVar(&input, "input", "{}", "Input object for --command as JSON, @file or @- for stdin")
	cmd.Flags().BoolVar(&mcp, "mcp", false, "Run --command with the MCP surface environment")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt of a destructive --command")
	return cmd
}

// checkTarget resolves the extension to check: a directory when target
// looks like a path, else an installed extension by name, else a directory
// of that name. A directory not named redmine-cli-<name> gets its name from
// the manifest (Name is left empty here).
func checkTarget(cmd *cobra.Command, opts Options, target string) (*extmgr.Extension, error) {
	pathLike := target == "." || strings.ContainsAny(target, `/\`) || strings.HasPrefix(target, ".")
	if !pathLike {
		m, err := opts.manager(cmd)
		if err != nil {
			return nil, err
		}
		ext, err := m.Get(target)
		if err == nil {
			return ext, nil
		}
		if info, statErr := os.Stat(target); statErr != nil || !info.IsDir() {
			if errors.Is(err, extmgr.ErrNotInstalled) {
				return nil, &cmdutil.CodedError{Code: output.ErrCodeNotFound, Message: err.Error(), Err: err}
			}
			return nil, err
		}
	}
	dir, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, &cmdutil.CodedError{Code: output.ErrCodeNotFound, Message: fmt.Sprintf("%s is not a directory", dir)}
	}
	ext := &extmgr.Extension{Dir: dir, Kind: extmgr.KindLocal}
	if name, ok := strings.CutPrefix(filepath.Base(dir), extmgr.Prefix); ok {
		ext.Name = name
		ext.Path = extmgr.ExecutablePath(dir, name)
	}
	return ext, nil
}

// readInputArg reads a JSON argument: literal text, @file, or @- (stdin).
func readInputArg(s string, stdin io.Reader) ([]byte, error) {
	path, ok := strings.CutPrefix(s, "@")
	if !ok {
		return []byte(s), nil
	}
	var r io.Reader
	if path == "-" {
		r = stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		r = file
	}
	data, err := io.ReadAll(io.LimitReader(r, spec.MaxJSONArgSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > spec.MaxJSONArgSize {
		return nil, fmt.Errorf("input is larger than %d bytes", spec.MaxJSONArgSize)
	}
	return bytes.TrimSpace(data), nil
}

func printCheck(printer output.Printer, ext *extmgr.Extension, path string) {
	res := checkResult{Name: ext.Name, File: path, Valid: true, Commands: []checkedCommand{}}
	usages := cliCommands(ext)
	for i := range ext.Manifest.Commands {
		c := &ext.Manifest.Commands[i]
		cc := checkedCommand{Name: c.Name, Usage: usages[i], Writes: c.Writes, Destructive: c.Destructive, Raw: c.Raw}
		if c.MCPEnabled() {
			cc.MCPTool = spec.ToolName(ext.Name, c.Name)
		}
		res.Commands = append(res.Commands, cc)
	}
	if printer.Format() == output.FormatJSON {
		printer.JSON(res)
		return
	}
	rows := make([][]string, 0, len(res.Commands))
	for _, c := range res.Commands {
		mode := "read"
		switch {
		case c.Raw:
			mode = "raw"
		case c.Destructive:
			mode = "destructive"
		case c.Writes:
			mode = "write"
		}
		rows = append(rows, []string{c.Usage, mode, c.MCPTool})
	}
	printer.Table([]string{"Command", "Mode", "MCP Tool"}, rows)
	printer.Success(fmt.Sprintf("%s is valid", path))
}
