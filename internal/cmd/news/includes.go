package news

import (
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// allowedGetIncludes lists the include keys accepted by GET /news/:id.json.
// reactions needs Redmine 7.1+.
var allowedGetIncludes = map[string]struct{}{
	"attachments": {},
	"comments":    {},
	"reactions":   {},
}

func validateGetIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedGetIncludes)
}

var completeGetIncludes = cmdutil.CompleteIncludes(allowedGetIncludes)
