package user

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
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
	return validateIncludes(values, allowedUserIncludes)
}

func validateUserListIncludes(values []string) error {
	return validateIncludes(values, allowedUserListIncludes)
}

func validateIncludes(values []string, allowed map[string]struct{}) error {
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := allowed[v]; !ok {
			return fmt.Errorf("unknown --include value %q (allowed: %s)", v, sortedIncludeKeys(allowed))
		}
	}
	return nil
}

func completeUserIncludes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return strings.Split(sortedIncludeKeys(allowedUserIncludes), ", "), cobra.ShellCompDirectiveNoFileComp
}

func completeUserListIncludes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return strings.Split(sortedIncludeKeys(allowedUserListIncludes), ", "), cobra.ShellCompDirectiveNoFileComp
}

func sortedIncludeKeys(m map[string]struct{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
