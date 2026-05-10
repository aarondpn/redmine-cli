package customfield

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdCustomFieldList(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all custom field definitions",
		Long: "List Redmine custom field definitions. Requires an administrator API " +
			"key because the underlying /custom_fields.json endpoint is admin-only.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching custom fields...")
			result, err := ops.ListCustomFields(context.Background(), client, struct{}{})
			stop()
			if err != nil {
				return err
			}

			cmdutil.RenderCollection(
				printer,
				result.CustomFields,
				[]string{"ID", "Name", "Type", "Format", "Required", "Filter", "Searchable", "Multiple"},
				func(cf models.CustomField, styled bool) []string {
					id := fmt.Sprintf("%d", cf.ID)
					if styled {
						id = output.StyleID.Render(id)
					}
					return []string{
						id,
						cf.Name,
						cf.CustomizedType,
						cf.FieldFormat,
						boolLabel(cf.IsRequired),
						boolLabel(cf.IsFilter),
						boolLabel(cf.Searchable),
						boolLabel(cf.Multiple),
					}
				},
			)
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
