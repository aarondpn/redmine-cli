package enumeration

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

// NewCmdDocumentCategories creates the document-categories command group.
func NewCmdDocumentCategories(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "document-categories",
		Aliases: []string{"document-category"},
		Short:   "List document categories",
	}
	cmd.AddCommand(newCmdDocumentCategoriesList(f))
	return cmd
}

func newCmdDocumentCategoriesList(f *cmdutil.Factory) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List all document categories",
		Example: `  # List document categories
  redmine document-categories list

  # As JSON
  redmine document-categories list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)
			stop := printer.Spinner("Fetching document categories...")
			result, err := ops.ListDocumentCategories(cmdutil.Context(cmd), client, struct{}{})
			stop()
			if err != nil {
				return err
			}
			renderEnumerations(printer, result.DocumentCategories, "document categories", true)
			return nil
		},
	}
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
