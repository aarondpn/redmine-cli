package wiki

import (
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// allowedGetIncludes lists the include keys accepted by
// GET /projects/:id/wiki/:page.json.
var allowedGetIncludes = map[string]struct{}{
	"attachments": {},
}

func validateGetIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedGetIncludes)
}

var completeGetIncludes = cmdutil.CompleteIncludes(allowedGetIncludes)
