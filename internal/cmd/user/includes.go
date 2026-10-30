package user

import (
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// allowedUserIncludes lists the include keys accepted by GET /users/:id.json
// and GET /users/current.json. Redmine 2.1+ supports groups and memberships;
// auth_source needs Redmine 6.0+ and an admin caller.
var allowedUserIncludes = map[string]struct{}{
	"auth_source": {},
	"groups":      {},
	"memberships": {},
}

// allowedUserListIncludes lists the include keys rendered by GET /users.json.
var allowedUserListIncludes = map[string]struct{}{
	"auth_source": {},
}

func validateUserIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedUserIncludes)
}

func validateUserListIncludes(values []string) error {
	return cmdutil.ValidateIncludes(values, allowedUserListIncludes)
}

var (
	completeUserIncludes     = cmdutil.CompleteIncludes(allowedUserIncludes)
	completeUserListIncludes = cmdutil.CompleteIncludes(allowedUserListIncludes)
)
