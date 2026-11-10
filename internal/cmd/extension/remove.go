package extension

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdRemove(f *cmdutil.Factory, opts Options) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove an installed extension",
		Long: `Remove an installed extension. For a local extension only the link is
removed; the linked directory is left alone.`,
		Example:           `  redmine extension remove pr`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeExtensionNames(opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := opts.manager(cmd)
			if err != nil {
				return err
			}
			if err := m.Remove(args[0]); err != nil {
				if errors.Is(err, extmgr.ErrNotInstalled) {
					return &cmdutil.CodedError{Code: output.ErrCodeNotFound, Message: err.Error(), Err: err}
				}
				return err
			}
			f.Printer("").Action(output.ActionDeleted, "extension", args[0], fmt.Sprintf("Removed extension %s", args[0]))
			return nil
		},
	}
}
