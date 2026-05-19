package user

import (
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

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

// FormatCustomFieldValues renders "Name: Value; ..." for custom field arrays.
// Exported so the my-account command group can reuse it.
func FormatCustomFieldValues(items []models.CustomFieldValue) string {
	parts := make([]string, len(items))
	for i, cf := range items {
		parts[i] = fmt.Sprintf("%s: %v", cf.Name, cf.Value)
	}
	return strings.Join(parts, "; ")
}
