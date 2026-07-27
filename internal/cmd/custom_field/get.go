package customfield

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdCustomFieldGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:               "get <id-or-name>",
		Aliases:           []string{"show", "view"},
		Short:             "Show custom field definition details",
		Long:              "Show details for a custom field definition. Accepts a numeric ID or field name.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cmdutil.CompleteCustomFields(f),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			id, err := resolver.ResolveCustomField(ctx, client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching custom field...")
			field, err := ops.GetCustomField(ctx, client, ops.GetCustomFieldInput{ID: id})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(field)
				return nil
			}
			if printer.Format() == output.FormatCSV {
				printer.CSV(
					[]string{"ID", "Name", "Type", "Format", "Required", "Filter", "Searchable", "Multiple", "Default", "Possible Values", "Trackers", "Roles", "For All", "Projects"},
					[][]string{{
						fmt.Sprintf("%d", field.ID),
						field.Name,
						field.CustomizedType,
						field.FieldFormat,
						boolLabel(field.IsRequired),
						boolLabel(field.IsFilter),
						boolLabel(field.Searchable),
						boolLabel(field.Multiple),
						field.DefaultValue,
						formatPossibleValues(field.PossibleValues),
						formatIDNames(field.Trackers),
						formatIDNames(field.Roles),
						optionalBoolLabel(field.IsForAll),
						formatIDNames(field.Projects),
					}},
				)
				return nil
			}

			details := []output.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", field.ID)},
				{Key: "Name", Value: field.Name},
				{Key: "Type", Value: field.CustomizedType},
				{Key: "Format", Value: field.FieldFormat},
				{Key: "Required", Value: boolLabel(field.IsRequired)},
				{Key: "Filter", Value: boolLabel(field.IsFilter)},
				{Key: "Searchable", Value: boolLabel(field.Searchable)},
				{Key: "Multiple", Value: boolLabel(field.Multiple)},
				{Key: "Visible", Value: boolLabel(field.Visible)},
			}
			if field.Description != "" {
				details = append(details, output.KeyValue{Key: "Description", Value: field.Description})
			}
			if field.Editable != nil {
				details = append(details, output.KeyValue{Key: "Editable", Value: boolLabel(*field.Editable)})
			}
			if field.IsForAll != nil {
				details = append(details, output.KeyValue{Key: "For All Projects", Value: boolLabel(*field.IsForAll)})
			}
			if field.DefaultValue != "" {
				details = append(details, output.KeyValue{Key: "Default", Value: field.DefaultValue})
			}
			if field.DefaultValueMode != "" {
				details = append(details, output.KeyValue{Key: "Default Mode", Value: field.DefaultValueMode})
			}
			if field.Regexp != "" {
				details = append(details, output.KeyValue{Key: "Regexp", Value: field.Regexp})
			}
			if field.MinLength != nil {
				details = append(details, output.KeyValue{Key: "Min Length", Value: fmt.Sprintf("%d", *field.MinLength)})
			}
			if field.MaxLength != nil {
				details = append(details, output.KeyValue{Key: "Max Length", Value: fmt.Sprintf("%d", *field.MaxLength)})
			}
			if value := formatPossibleValues(field.PossibleValues); value != "" {
				details = append(details, output.KeyValue{Key: "Possible Values", Value: value})
			}
			if value := formatIDNames(field.Trackers); value != "" {
				details = append(details, output.KeyValue{Key: "Trackers", Value: value})
			}
			if value := formatIDNames(field.Roles); value != "" {
				details = append(details, output.KeyValue{Key: "Roles", Value: value})
			}
			if value := formatIDNames(field.Projects); value != "" {
				details = append(details, output.KeyValue{Key: "Projects", Value: value})
			}

			printer.Detail(details)
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
