package customfield

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// NewCmdCustomFields creates the custom-fields command group.
func NewCmdCustomFields(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "custom-fields",
		Aliases: []string{"custom-field"},
		Short:   "Discover custom field definitions",
		Long: "List and inspect Redmine custom field definitions exposed by " +
			"the admin-only /custom_fields.json endpoint.",
	}

	cmd.AddCommand(newCmdCustomFieldList(f))
	cmd.AddCommand(newCmdCustomFieldGet(f))
	return cmd
}

func boolLabel(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// optionalBoolLabel renders a tri-state flag: servers older than Redmine 7.0
// omit is_for_all/editable entirely, and a blank cell is the honest answer
// there rather than "no".
func optionalBoolLabel(v *bool) string {
	if v == nil {
		return ""
	}
	return boolLabel(*v)
}

func formatPossibleValues(values []models.CustomFieldPossibleValue) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		label := v.Label
		if label == "" || label == v.Value {
			parts = append(parts, v.Value)
			continue
		}
		parts = append(parts, v.Value+" ("+label+")")
	}
	return strings.Join(parts, ", ")
}

func formatIDNames(items []models.IDName) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Name+" (ID: "+strconv.Itoa(item.ID)+")")
	}
	return strings.Join(parts, ", ")
}
