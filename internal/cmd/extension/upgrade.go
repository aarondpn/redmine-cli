package extension

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdUpgrade(f *cmdutil.Factory, opts Options) *cobra.Command {
	var (
		all     bool
		dryRun  bool
		upForce bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade [<name> | --all]",
		Short: "Upgrade installed extensions",
		Long: `Upgrade an extension, or all of them with --all.

A binary extension is reinstalled from its latest release when the tag
differs from the installed one; a git extension is fast-forwarded to its
upstream with 'git pull --ff-only'. Pinned and local extensions are skipped.

A binary extension installed from a checksum-verified release is only
upgraded to a release that can be verified too, unless --force is given.`,
		Example: `  redmine extension upgrade pr
  redmine extension upgrade --all --dry-run`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeExtensionNames(opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all == (len(args) == 1) {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: "specify an extension name or --all"}
			}
			m, err := opts.manager(cmd)
			if err != nil {
				return err
			}
			printer := f.Printer("")
			ctx := cmdutil.Context(cmd)
			upOpts := extmgr.UpgradeOptions{DryRun: dryRun, Force: upForce}

			var results []*extmgr.UpgradeResult
			stop := printer.Spinner("Checking for extension upgrades...")
			if all {
				results, err = m.UpgradeAll(ctx, upOpts)
			} else {
				var res *extmgr.UpgradeResult
				if res, err = m.Upgrade(ctx, args[0], upOpts); err == nil {
					results = []*extmgr.UpgradeResult{res}
				}
			}
			stop()
			if err != nil {
				if errors.Is(err, extmgr.ErrNotInstalled) {
					return &cmdutil.CodedError{Code: output.ErrCodeNotFound, Message: err.Error(), Err: err}
				}
				return err
			}
			if results == nil {
				results = []*extmgr.UpgradeResult{}
			}
			if all && cmdutil.HandleEmpty(printer, results, "extensions") {
				return nil
			}

			failed := 0
			for _, r := range results {
				if r.Status == extmgr.StatusFailed {
					failed++
				}
				for _, w := range r.Warnings {
					printer.Warning(r.Name + ": " + w)
				}
			}
			if printer.Format() == output.FormatJSON {
				printer.JSON(results)
			} else {
				rows := make([][]string, 0, len(results))
				for _, r := range results {
					rows = append(rows, []string{r.Name, statusText(r.Status), r.From, r.To, r.Note})
				}
				printer.Table([]string{"Name", "Status", "From", "To", "Note"}, rows)
			}
			if failed > 0 {
				return fmt.Errorf("%d extension(s) failed to upgrade", failed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Upgrade every installed extension")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only report available upgrades")
	cmd.Flags().BoolVar(&upForce, "force", false, "Reinstall binaries even when current or no longer checksum-verified; reset git extensions to upstream, discarding local changes")
	return cmd
}

func statusText(s extmgr.UpgradeStatus) string {
	switch s {
	case extmgr.StatusUpgraded:
		return "upgraded"
	case extmgr.StatusUpToDate:
		return "up to date"
	case extmgr.StatusAvailable:
		return "upgrade available"
	case extmgr.StatusSkipped:
		return "skipped"
	case extmgr.StatusFailed:
		return "failed"
	}
	return string(s)
}
