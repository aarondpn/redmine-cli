package project

import (
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// allowedIncludes lists the include keys accepted by /projects.json and
// /projects/<id>.json. Keep alphabetical for stable error messages.
var allowedIncludes = map[string]struct{}{
	"enabled_modules":       {},
	"issue_categories":      {},
	"issue_custom_fields":   {},
	"time_entry_activities": {},
	"trackers":              {},
}

func validateProjectIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedIncludes)
}

var completeProjectIncludes = cmdutil.CompleteIncludes(allowedIncludes)
