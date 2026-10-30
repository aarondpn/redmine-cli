package issue

import (
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// allowedGetIncludes lists the include keys rendered by GET /issues/:id.json.
// allowed_statuses needs Redmine 5.0+ and reactions needs Redmine 7.1+;
// older servers ignore them.
var allowedGetIncludes = map[string]struct{}{
	"allowed_statuses": {},
	"attachments":      {},
	"changesets":       {},
	"children":         {},
	"journals":         {},
	"reactions":        {},
	"relations":        {},
	"watchers":         {},
}

// allowedListIncludes lists the include keys rendered by GET /issues.json.
var allowedListIncludes = map[string]struct{}{
	"attachments": {},
	"relations":   {},
}

func validateGetIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedGetIncludes)
}

func validateListIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedListIncludes)
}

var (
	completeGetIncludes  = cmdutil.CompleteIncludes(allowedGetIncludes)
	completeListIncludes = cmdutil.CompleteIncludes(allowedListIncludes)
)
