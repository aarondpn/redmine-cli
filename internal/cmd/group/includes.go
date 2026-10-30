package group

import (
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// allowedGetIncludes lists the include keys accepted by GET /groups/:id.json.
var allowedGetIncludes = map[string]struct{}{
	"memberships": {},
	"users":       {},
}

func validateGetIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedGetIncludes)
}

var completeGetIncludes = cmdutil.CompleteIncludes(allowedGetIncludes)
