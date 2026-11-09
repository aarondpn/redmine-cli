package user

import (
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// AppendAccountDates appends the account timestamps and security metadata
// shared by `users get` and `users me`. Updated, Password Changed and
// Two-factor only appear when Redmine sent them.
func AppendAccountDates(pairs []output.KeyValue, u *models.User) []output.KeyValue {
	pairs = append(pairs, output.KeyValue{Key: "Created", Value: u.CreatedOn})
	if u.UpdatedOn != "" {
		pairs = append(pairs, output.KeyValue{Key: "Updated", Value: u.UpdatedOn})
	}
	pairs = append(pairs, output.KeyValue{Key: "Last Login", Value: u.LastLoginOn})
	if u.PasswdChangedOn != "" {
		pairs = append(pairs, output.KeyValue{Key: "Password Changed", Value: u.PasswdChangedOn})
	}
	if u.TwofaScheme != "" {
		pairs = append(pairs, output.KeyValue{Key: "Two-Factor", Value: u.TwofaScheme})
	}
	return pairs
}

// formatIDNameList renders "Name (#ID), ..." for groups and similar arrays.
func formatIDNameList(items []models.IDName) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = fmt.Sprintf("%s (#%d)", it.Name, it.ID)
	}
	return strings.Join(parts, ", ")
}

// formatUserMembershipList renders "Project (#ID) [Role1, Role2]; ..." for
// the memberships include.
func formatUserMembershipList(items []models.UserMembership) string {
	parts := make([]string, len(items))
	for i, m := range items {
		entry := fmt.Sprintf("%s (#%d)", m.Project.Name, m.Project.ID)
		if len(m.Roles) > 0 {
			roleNames := make([]string, len(m.Roles))
			for j, r := range m.Roles {
				roleNames[j] = r.Name
			}
			entry += " [" + strings.Join(roleNames, ", ") + "]"
		}
		parts[i] = entry
	}
	return strings.Join(parts, "; ")
}

// FormatCustomFieldValues renders "Name: Value; ..." for custom field arrays,
// skipping fields without a value. Exported so the my-account command group
// can reuse it.
func FormatCustomFieldValues(items []models.CustomFieldValue) string {
	return cmdutil.FormatCustomFields(items)
}
