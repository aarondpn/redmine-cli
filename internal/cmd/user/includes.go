package user

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// allowedUserIncludes lists the include keys accepted by GET /users/:id.json
// and GET /users/current.json. Redmine 2.1+ supports both keys.
var allowedUserIncludes = map[string]struct{}{
	"groups":      {},
	"memberships": {},
}

func validateUserIncludes(values []string) error {
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := allowedUserIncludes[v]; !ok {
			return fmt.Errorf("unknown --include value %q (allowed: %s)", v, sortedIncludeKeys(allowedUserIncludes))
		}
	}
	return nil
}

func completeUserIncludes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return strings.Split(sortedIncludeKeys(allowedUserIncludes), ", "), cobra.ShellCompDirectiveNoFileComp
}

func sortedIncludeKeys(m map[string]struct{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
