package extension

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdExec(f *cmdutil.Factory, opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <name> [args...]",
		Short: "Run an extension, even if a core command has the same name",
		Long: `Run an installed extension exactly like 'redmine <name> [args...]'.

Core commands always win over extensions of the same name; exec is the way
to run such a shadowed extension. Global flags go before <name>.`,
		Example:            `  redmine extension exec pr list 123`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		ValidArgsFunction:  completeExtensionNames(opts),
		Annotations:        map[string]string{annotationExec: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			global, rest := splitGlobalFlags(root.PersistentFlags(), args)
			if len(rest) == 0 {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: "missing extension name; usage: " + cmd.UseLine()}
			}
			if rest[0] == "-h" || rest[0] == "--help" {
				return cmd.Help()
			}
			m, err := opts.manager(cmd)
			if err != nil {
				return err
			}
			ext, err := m.Get(rest[0])
			if err != nil {
				if errors.Is(err, extmgr.ErrNotInstalled) {
					return &cmdutil.CodedError{Code: output.ErrCodeNotFound, Message: err.Error(), Err: err}
				}
				return err
			}

			// Run the extension's command tree under a throwaway root that
			// shares the real root's global flags (the same *pflag.Flag
			// values) and hooks, so flags, help, validation and rendering
			// behave exactly as for 'redmine <name>'.
			shadow := &cobra.Command{
				Use:                root.Name(),
				SilenceUsage:       true,
				SilenceErrors:      true,
				PersistentPreRunE:  root.PersistentPreRunE,
				PersistentPostRunE: root.PersistentPostRunE,
			}
			shadow.PersistentFlags().AddFlagSet(root.PersistentFlags())
			shadow.AddCommand(NewExtensionCommand(f, ext, opts))
			shadowArgs := append(append(global, ext.Name), rest[1:]...)
			shadow.SetArgs(shadowArgs)
			shadow.SetIn(cmd.InOrStdin())
			shadow.SetOut(cmd.OutOrStdout())
			shadow.SetErr(cmd.ErrOrStderr())
			return shadow.ExecuteContext(WithInvocationArgs(cmdutil.Context(cmd), shadowArgs))
		},
	}
	cmdutil.MarkNoJSONOutput(cmd)
	return cmd
}
