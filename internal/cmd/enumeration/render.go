// Package enumeration implements the priorities, activities and
// document-categories command groups.
package enumeration

import (
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// renderEnumerations prints an enumeration list. With withFlags the Default and
// Active columns are included; project-scoped activity tables only show id and name.
func renderEnumerations(printer output.Printer, items []models.Enumeration, noun string, withFlags bool) {
	if cmdutil.HandleEmpty(printer, items, noun) {
		return
	}
	headers := []string{"ID", "Name"}
	if withFlags {
		headers = append(headers, "Default", "Active")
	}
	cmdutil.RenderCollection(printer, items, headers, func(e models.Enumeration) []string {
		row := []string{output.StyleID.Render(strconv.Itoa(e.ID)), e.Name}
		if withFlags {
			row = append(row, yesNo(e.IsDefault), yesNo(e.Active))
		}
		return row
	})
}
