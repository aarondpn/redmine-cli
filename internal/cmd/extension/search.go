package extension

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdSearch(f *cmdutil.Factory, opts Options) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search GitHub for extensions",
		Long: `Search GitHub for repositories with the topic ` + extmgr.Topic + `,
most starred first. Installed extensions are marked. The query uses GitHub
search syntax and narrows the results.`,
		Example: `  redmine extension search
  redmine extension search pull request
  redmine extension search --jq '.[] | select(.installed | not) | .repo'`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > extmgr.MaxSearchResults {
				return &cmdutil.CodedError{Code: output.ErrCodeValidationFailed, Message: "--limit must be between 1 and " + strconv.Itoa(extmgr.MaxSearchResults)}
			}
			m, err := opts.manager(cmd)
			if err != nil {
				return err
			}
			printer := f.Printer("")
			stop := printer.Spinner("Searching GitHub...")
			results, err := m.Search(cmdutil.Context(cmd), strings.Join(args, " "), limit)
			stop()
			if err != nil {
				return err
			}
			if cmdutil.HandleEmpty(printer, results, "extensions") {
				return nil
			}
			if printer.Format() == output.FormatJSON {
				printer.JSON(results)
				return nil
			}
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				installed := ""
				if r.Installed {
					installed = "installed"
				}
				rows = append(rows, []string{r.Repo, r.Name, strconv.Itoa(r.Stars), installed, r.Description})
			}
			printer.Table([]string{"Repo", "Name", "Stars", "Installed", "Description"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum number of results (at most 100)")
	return cmd
}
