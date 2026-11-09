package extension

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

const (
	// annotationExtension marks every command an extension contributes;
	// the value is the extension name.
	annotationExtension = "redmine-cli/extension"
	// annotationPassthrough marks commands that run the extension with the
	// terminal attached (raw commands, extensions without a manifest).
	annotationPassthrough = "redmine-cli/extension-passthrough"
	// annotationExec marks `extension exec`, whose target decides whether
	// the terminal is attached.
	annotationExec = "redmine-cli/extension-exec"

	groupCore       = "core"
	groupExtensions = "extensions"

	// errCodeExtensionFailed is the JSON error code of an extension that
	// exited with a non-zero status.
	errCodeExtensionFailed = "extension_failed"

	// noticeWait is how long a finished command waits for the update check.
	noticeWait = time.Second
	// noticeTimeout bounds the background update check itself.
	noticeTimeout = 5 * time.Second
)

// IsExtensionCommand reports whether c was contributed by an extension.
func IsExtensionCommand(c *cobra.Command) bool {
	_, ok := c.Annotations[annotationExtension]
	return ok
}

// Register adds a top-level command for every extension in exts to root,
// under an "Extension Commands" help group, and returns them. An extension
// whose name is taken by a core command is skipped, since core commands
// always win; `redmine extension exec <name>` still runs it.
func Register(root *cobra.Command, f *cmdutil.Factory, exts []*extmgr.Extension, opts Options) []*cobra.Command {
	taken := map[string]bool{"help": true}
	for _, n := range CoreCommandNames(root) {
		taken[n] = true
	}
	var added []*cobra.Command
	for _, ext := range exts {
		if taken[ext.Name] {
			continue
		}
		taken[ext.Name] = true
		c := NewExtensionCommand(f, ext, opts)
		c.GroupID = groupExtensions
		added = append(added, c)
	}
	if len(added) == 0 {
		return nil
	}
	// Once a root has groups, cobra lists ungrouped commands as
	// "Additional Commands", so the core commands get a group of their own.
	root.AddGroup(
		&cobra.Group{ID: groupCore, Title: "Available Commands:"},
		&cobra.Group{ID: groupExtensions, Title: "Extension Commands:"},
	)
	for _, c := range root.Commands() {
		if c.GroupID == "" {
			c.GroupID = groupCore
		}
	}
	root.SetHelpCommandGroupID(groupCore)
	root.SetCompletionCommandGroupID(groupCore)
	root.AddCommand(added...)
	return added
}

// NewExtensionCommand builds the command tree of one extension: a stub that
// reports the problem for an unusable extension, a passthrough command for
// one without a manifest, and otherwise a parent with one subcommand per
// manifest command.
func NewExtensionCommand(f *cmdutil.Factory, ext *extmgr.Extension, opts Options) *cobra.Command {
	if ext.Err != nil {
		return brokenCommand(ext)
	}
	if ext.Manifest == nil {
		return passthroughCommand(f, ext, nil, opts)
	}
	parent := &cobra.Command{
		Use:         ext.Name,
		Short:       describe(ext),
		Long:        output.SanitizeTerminal(ext.Manifest.Description + "\n\n" + origin(ext)),
		Annotations: map[string]string{annotationExtension: ext.Name},
	}
	for i := range ext.Manifest.Commands {
		c := &ext.Manifest.Commands[i]
		if c.Raw {
			parent.AddCommand(passthroughCommand(f, ext, c, opts))
		} else {
			parent.AddCommand(declarativeCommand(f, ext, c, opts))
		}
	}
	return parent
}

// origin describes where an extension comes from, for help texts.
func origin(ext *extmgr.Extension) string {
	parts := []string{"Extension " + ext.Name}
	if repo := ext.Repo(); repo != "" {
		parts = append(parts, "from "+repo)
	}
	if v := ext.Version(); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, " ") + " (" + string(ext.Kind) + ")."
}

// brokenCommand stands in for an extension that cannot run, so a broken
// manifest is reported when the extension is used instead of breaking the
// whole CLI. Its flags are parsed, with unknown ones (meant for the
// extension) ignored, so `-o json` or `--jq` anywhere still turns the
// error into a JSON envelope.
func brokenCommand(ext *extmgr.Extension) *cobra.Command {
	return &cobra.Command{
		Use:                ext.Name,
		Short:              "Unavailable extension (run it to see why)",
		Args:               cobra.ArbitraryArgs,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		Annotations:        map[string]string{annotationExtension: ext.Name},
		RunE: func(cmd *cobra.Command, args []string) error {
			return brokenError(ext)
		},
	}
}

func brokenError(ext *extmgr.Extension) error {
	hint := fmt.Sprintf("Upgrade it with 'redmine extension upgrade %s' or remove it with 'redmine extension remove %s'.", ext.Name, ext.Name)
	var ve *spec.ValidationError
	if errors.As(ext.Err, &ve) {
		return cmdutil.WithHint(&cmdutil.CodedError{
			Code:    output.ErrCodeValidationFailed,
			Message: fmt.Sprintf("extension %s has an invalid %s", ext.Name, spec.FileName),
			Details: ve.Details(),
			Err:     ext.Err,
		}, hint)
	}
	return cmdutil.WithHint(fmt.Errorf("extension %s cannot be used: %w", ext.Name, ext.Err), hint)
}

// passthroughCommand runs the executable with the terminal attached, gh
// style: the command c of a manifest when it is raw, or the whole extension
// when it has no manifest (c == nil).
func passthroughCommand(f *cmdutil.Factory, ext *extmgr.Extension, c *spec.Command, opts Options) *cobra.Command {
	use, short := ext.Name, describe(ext)
	if c != nil {
		use, short = c.Name, output.SanitizeTerminal(c.Description)
	}
	cmd := &cobra.Command{
		Use:   use + " [args...]",
		Short: short,
		Long: short + "\n\nAll arguments are passed to the extension unchanged. redmine-cli global flags " +
			"(--profile, --server, ...) go before '" + use + "'; after it they are the extension's arguments.",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		Annotations:        map[string]string{annotationExtension: ext.Name, annotationPassthrough: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPassthrough(cmd, f, ext, c, args, opts)
		},
	}
	cmdutil.MarkNoJSONOutput(cmd)
	return cmd
}

// declarativeCommand derives a CLI command from a manifest command.
func declarativeCommand(f *cmdutil.Factory, ext *extmgr.Extension, c *spec.Command, opts Options) *cobra.Command {
	cmd := &cobra.Command{Annotations: map[string]string{annotationExtension: ext.Name}}
	if err := spec.Configure(cmd, c); err != nil {
		// Validation rules out every cause; keep the CLI usable anyway.
		return &cobra.Command{
			Use:         c.Name,
			Short:       output.SanitizeTerminal(c.Description),
			Args:        cobra.ArbitraryArgs,
			Annotations: cmd.Annotations,
			RunE: func(*cobra.Command, []string) error {
				return fmt.Errorf("extension %s: command %s: %w", ext.Name, c.Name, err)
			},
		}
	}
	var notes []string
	if c.Writes {
		notes = append(notes, "Modifies Redmine data.")
	} else {
		notes = append(notes, "Read-only: runs with REDMINE_READ_ONLY=1.")
	}
	var yes bool
	if c.Destructive {
		cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
		notes = append(notes, "Destructive: asks for confirmation unless --yes is given.")
	}
	if c.MCPEnabled() {
		notes = append(notes, "MCP tool: "+spec.ToolName(ext.Name, c.Name)+".")
	}
	cmd.Long = output.SanitizeTerminal(c.Description + "\n\n" + strings.Join(notes, " ") + "\n" + origin(ext))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		input, err := spec.BuildInput(cmd, c, args)
		if err != nil {
			return inputError(err)
		}
		label := cmd.CommandPath()
		if c.Destructive && !yes {
			ok, err := confirmDestructive(f, label)
			if err != nil || !ok {
				return err
			}
		}
		return runDeclarative(cmd, f, ext, c, input, runner.SurfaceCLI, label, opts)
	}
	return cmd
}

// inputError reports invalid arguments as a validation failure.
func inputError(err error) error {
	var ve *spec.ValidationError
	if errors.As(err, &ve) {
		return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: "invalid input", Details: ve.Details(), Err: err}
	}
	return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: err.Error(), Err: err}
}

// confirmDestructive asks before a destructive command runs. Without a
// terminal to ask on, --yes is required.
func confirmDestructive(f *cmdutil.Factory, label string) (bool, error) {
	if !stdinIsTerminal(f) {
		return false, &cmdutil.CodedError{
			Code:    output.ErrCodeValidationFailed,
			Message: label + " is destructive and needs confirmation; pass --yes (there is no terminal to ask on)",
		}
	}
	if cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, label+" is destructive. Continue?") {
		return true, nil
	}
	f.Printer("").Outcome(false, "run", "extension_command", label, "Cancelled")
	return false, nil
}

// warnAPIKey tells the user that --api-key / --api-key-file do not reach the
// extension: credentials are never handed to extension processes.
func warnAPIKey(f *cmdutil.Factory) {
	if f.APIKeyOverride != "" || f.APIKeyFile != "" {
		f.Warn("--api-key and --api-key-file are not passed to extensions; the redmine calls an extension makes use the profile's credentials (or REDMINE_API_KEY)")
	}
}

// commandEnv is the environment contract for running command of ext.
func commandEnv(f *cmdutil.Factory, ext *extmgr.Extension, command string, surface runner.Surface) (runner.Env, error) {
	env, err := runner.FromFactory(f)
	if err != nil {
		return runner.Env{}, err
	}
	env.Extension = ext.Name
	env.Command = command
	env.Surface = surface
	return env, nil
}

// runDeclarative runs a declarative command with an already validated input
// and renders its result like a core command: JSON for -o json / --jq, the
// manifest's table or detail spec otherwise. label names the command in
// messages.
func runDeclarative(cmd *cobra.Command, f *cmdutil.Factory, ext *extmgr.Extension, c *spec.Command, input map[string]any, surface runner.Surface, label string, opts Options) error {
	return runDeclarativeWith(cmd, f, ext, c, input, surface, label, opts, true)
}

// runDeclarativeWith is runDeclarative with the update notice optional
// (`extension check` is a development tool and skips it).
func runDeclarativeWith(cmd *cobra.Command, f *cmdutil.Factory, ext *extmgr.Extension, c *spec.Command, input map[string]any, surface runner.Surface, label string, opts Options, notice bool) error {
	warnAPIKey(f)
	env, err := commandEnv(f, ext, c.Name, surface)
	if err != nil {
		return err
	}
	env.ReadOnly = env.ReadOnly || !c.Writes
	if surface == runner.SurfaceMCP {
		env.Tool = spec.ToolName(ext.Name, c.Name)
	}
	printer := f.Printer("")
	if notice {
		defer startNotice(f, ext, opts)()
	}

	ctx := cmdutil.Context(cmd)
	// The extension's stderr is streamed live. At a terminal it is
	// sanitized like any other text redmine-cli shows; redirected, the
	// bytes are passed on unchanged.
	stderr := f.IOStreams.ErrOut
	var tw *terminalWriter
	if stderrIsTerminal(f) {
		tw = &terminalWriter{w: f.IOStreams.ErrOut}
		stderr = tw
	}
	res, err := runner.RunCommand(ctx, ext.Path, c, input, env.Environ(os.Environ()), stderr)
	if tw != nil {
		tw.Flush()
	}
	if err != nil {
		return runFailure(f, printer, label, res, err)
	}
	return renderResult(ctx, f, printer, label, c, res.Stdout)
}

// runFailure turns a failed run into the command's error. A non-zero exit
// is propagated as the exit code; the extension's stderr, streamed live,
// already explained it.
func runFailure(f *cmdutil.Factory, printer output.Printer, label string, res *runner.Result, err error) error {
	var (
		exitErr *runner.ExitError
		timeout *runner.TimeoutError
	)
	switch {
	case errors.As(err, &exitErr):
		msg := fmt.Sprintf("%s exited with status %d", label, exitErr.Code)
		if printer.Format() == output.FormatJSON {
			if exitErr.Message != "" {
				msg = label + " failed: " + exitErr.Message
			}
			_ = output.RenderErrorJSON(f.IOStreams.ErrOut, output.ErrorEnvelope{Error: output.ErrorDetail{Message: msg, Code: errCodeExtensionFailed}})
		} else if res == nil || len(bytes.TrimSpace(res.Stderr)) == 0 {
			fmt.Fprintf(f.IOStreams.ErrOut, "Error: %s\n", msg)
		}
		return &cmdutil.SilentError{Code: exitErr.Code}
	case errors.As(err, &timeout):
		return &cmdutil.CodedError{Code: output.ErrCodeTimeout, Message: label + " " + timeout.Error(), Err: err}
	default:
		return fmt.Errorf("%s: %w", label, err)
	}
}

// renderResult prints a command's stdout. Non-JSON output is shown as text
// (as a JSON string in JSON mode, so stdout stays JSON).
func renderResult(ctx context.Context, f *cmdutil.Factory, printer output.Printer, label string, c *spec.Command, stdout []byte) error {
	jsonMode := printer.Format() == output.FormatJSON
	v, ok := runner.DecodeOutput(stdout)
	if !ok {
		text := string(stdout)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		if jsonMode {
			// Like shell command substitution, drop the final newline.
			printer.JSON(strings.TrimRight(text, "\r\n"))
			return nil
		}
		text = output.SanitizeTerminal(text)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		fmt.Fprint(f.IOStreams.Out, text)
		return nil
	}
	if jsonMode {
		printer.JSON(v)
		return nil
	}
	r, err := spec.Render(ctx, c.Output, v)
	if err != nil {
		f.Warn(fmt.Sprintf("%s: the output spec failed (%v); showing JSON", label, err))
		printer.JSON(v)
		return nil
	}
	switch r.View {
	case spec.ViewTable:
		printer.Table(r.Headers, r.Rows)
	case spec.ViewDetail:
		printer.Detail(r.Pairs)
	default:
		printer.JSON(v)
	}
	return nil
}

// runPassthrough runs a raw command (c != nil) or an extension without a
// manifest with stdio attached and propagates its exit code.
func runPassthrough(cmd *cobra.Command, f *cmdutil.Factory, ext *extmgr.Extension, c *spec.Command, args []string, opts Options) error {
	// Cobra does not parse flags here, so the global flags in front of the
	// command are applied by hand. Everything after the command goes to the
	// extension unchanged, even arguments that look like global flags.
	fs := cmd.Root().PersistentFlags()
	global, rest := passthroughArgs(cmd, args)
	if len(global) > 0 {
		if err := fs.Parse(global); err != nil {
			return err
		}
		if err := cmdutil.ApplyGlobalFlags(f, cmd); err != nil {
			return err
		}
	}
	if jq := fs.Lookup("jq"); jq != nil && jq.Changed {
		return fmt.Errorf("--jq is not supported for %s: it does not print JSON", cmd.CommandPath())
	}
	warnAPIKey(f)

	command := ""
	if c != nil {
		command = c.Name
	}
	env, err := commandEnv(f, ext, command, runner.SurfaceCLI)
	if err != nil {
		return err
	}
	argv := rest
	var timeout time.Duration
	if c != nil {
		env.ReadOnly = env.ReadOnly || !c.Writes
		argv = append([]string{c.Name}, rest...)
		// Raw commands are often interactive: only an explicit timeout
		// applies, not the default for declarative commands.
		if c.Timeout != "" {
			timeout = c.TimeoutDuration()
		}
	}
	defer startNotice(f, ext, opts)()

	code, err := runner.RunAttached(cmdutil.Context(cmd), runner.AttachedRequest{
		Path:    ext.Path,
		Args:    argv,
		Env:     env.Environ(os.Environ()),
		Stdin:   f.IOStreams.In,
		Stdout:  f.IOStreams.Out,
		Stderr:  f.IOStreams.ErrOut,
		Timeout: timeout,
	})
	if err != nil {
		var te *runner.TimeoutError
		if errors.As(err, &te) {
			return &cmdutil.CodedError{Code: output.ErrCodeTimeout, Message: cmd.CommandPath() + " " + te.Error(), Err: err}
		}
		return fmt.Errorf("%s: %w", cmd.CommandPath(), err)
	}
	if code != 0 {
		return &cmdutil.SilentError{Code: code}
	}
	return nil
}

// AttachesTerminal reports whether args run an extension with the terminal
// attached, directly or through `extension exec`. Such an extension shares
// the terminal's process group and gets Ctrl-C itself, so the caller must
// not end the process on it but wait for the extension to exit.
func AttachesTerminal(root *cobra.Command, args []string, opts Options) bool {
	target, rest, err := root.Find(args)
	if err != nil || target == nil {
		return false
	}
	if target.Annotations[annotationPassthrough] == "true" {
		return true
	}
	if target.Annotations[annotationExec] != "true" {
		return false
	}
	_, rest = splitGlobalFlags(root.PersistentFlags(), rest)
	if len(rest) == 0 {
		return false
	}
	m, err := opts.manager(nil)
	if err != nil {
		return false
	}
	ext, err := m.Get(rest[0])
	if err != nil || ext.Err != nil {
		return false
	}
	if ext.Manifest == nil {
		return true
	}
	if len(rest) < 2 {
		return false
	}
	c := ext.Manifest.Command(rest[1])
	return c != nil && c.Raw
}

// startNotice starts the background update check of ext and returns the
// function that prints its result once the command is done. Only installed
// (not local, not pinned) extensions run at a terminal are checked, and
// REDMINE_NO_UPDATE_CHECK turns it off.
func startNotice(f *cmdutil.Factory, ext *extmgr.Extension, opts Options) func() {
	if !extmgr.NoticeEligible(ext) || updateCheckDisabled() || !stderrIsTerminal(f) {
		return func() {}
	}
	m, err := opts.manager(nil)
	if err != nil {
		return func() {}
	}
	done := make(chan *extmgr.Notice, 1)
	ctx, cancel := context.WithTimeout(context.Background(), noticeTimeout)
	go func() {
		n, _ := m.CheckForUpdate(ctx, ext)
		done <- n
	}()
	return func() {
		defer cancel()
		select {
		case n := <-done:
			printNotice(f, n)
		case <-time.After(noticeWait):
		}
	}
}

func updateCheckDisabled() bool {
	v := os.Getenv(runner.EnvNoUpdateCheck)
	return v == "1" || strings.EqualFold(v, "true")
}

func printNotice(f *cmdutil.Factory, n *extmgr.Notice) {
	if n == nil {
		return
	}
	fmt.Fprintf(f.IOStreams.ErrOut, "\nA new release of %s is available: %s → %s\nRun \"redmine extension upgrade %s\" to upgrade\n",
		n.Name, output.SanitizeTerminal(n.Current), output.SanitizeTerminal(n.Latest), n.Name)
}
