package extension

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdCreate(f *cmdutil.Factory, opts Options) *cobra.Command {
	var precompiled string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new extension",
		Long: `Create the directory redmine-cli-<name> with a working extension:
the executable (a shell script, or a Go program with --precompiled=go), the
manifest redmine-extension.yml with a schema modeline for editor validation,
.vscode/settings.json mapping the schema, and a README. A Go extension also
gets a GitHub Actions release workflow that publishes the binaries
'redmine extension install' looks for.`,
		Example: `  redmine extension create hello
  redmine extension create hello --precompiled=go`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := spec.ValidateExtensionName(name); err != nil {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: err.Error(), Err: err}
			}
			if slices.Contains(CoreCommandNames(cmd.Root()), name) {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: fmt.Sprintf("extension name %q is already taken by a core command", name)}
			}
			if precompiled != "" && precompiled != "go" {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: fmt.Sprintf("--precompiled %q is not supported; use --precompiled=go or leave it out for a script extension", precompiled)}
			}
			dir := extmgr.Prefix + name
			if _, err := os.Lstat(dir); err == nil {
				return &cmdutil.CodedError{Code: output.ErrCodeAlreadyExists, Message: fmt.Sprintf("%s already exists", dir)}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := writeScaffold(dir, scaffoldFiles(name, precompiled)); err != nil {
				return err
			}

			next := fmt.Sprintf("cd %s && redmine extension install . && redmine %s whoami", dir, name)
			if precompiled == "go" {
				next = fmt.Sprintf("cd %s && go build && redmine extension install . && redmine %s whoami", dir, name)
			}
			f.Printer("").Action(output.ActionCreated, "extension", name,
				fmt.Sprintf("Created %s/. Next: %s", dir, next))
			return nil
		},
	}
	cmd.Flags().StringVar(&precompiled, "precompiled", "", `Create a precompiled extension in this language ("go")`)
	_ = cmd.RegisterFlagCompletionFunc("precompiled", cobra.FixedCompletions([]string{"go"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

// writeScaffold writes files into a staging directory next to dir and
// renames it into place, so a failure leaves nothing half-created.
func writeScaffold(dir string, files []scaffoldFile) error {
	stage, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, file := range files {
		path := filepath.Join(stage, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.Content), file.Mode); err != nil {
			return err
		}
	}
	// MkdirTemp creates 0700; a project directory is normally 0755.
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	return os.Rename(stage, dir)
}
