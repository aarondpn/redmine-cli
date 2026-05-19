package user

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// AllowedMailNotifications lists the Redmine mail notification preference
// values. The wiki documents only "only_my_events" and "none"; the full set
// comes from app/models/user.rb in Redmine.
var AllowedMailNotifications = map[string]struct{}{
	"all":            {},
	"only_my_events": {},
	"only_assigned":  {},
	"only_owner":     {},
	"none":           {},
}

// ValidateMailNotification accepts the empty string (meaning "unset") and any
// value from AllowedMailNotifications. Used by both the users and my-account
// command groups to fail fast before hitting the API.
func ValidateMailNotification(value string) error {
	if value == "" {
		return nil
	}
	if _, ok := AllowedMailNotifications[value]; !ok {
		return fmt.Errorf("invalid --mail-notification %q (allowed: %s)", value, sortedMailNotificationKeys())
	}
	return nil
}

// CompleteMailNotification provides shell completion for --mail-notification.
func CompleteMailNotification(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return strings.Split(sortedMailNotificationKeys(), ", "), cobra.ShellCompDirectiveNoFileComp
}

func sortedMailNotificationKeys() string {
	keys := make([]string, 0, len(AllowedMailNotifications))
	for k := range AllowedMailNotifications {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
