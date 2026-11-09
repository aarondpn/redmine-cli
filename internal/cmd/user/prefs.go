package user

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// Flag help shared by users create/update and my-account update, so the
// same concept reads the same everywhere.
const (
	prefFlagUsage = "User preference as key=value (repeatable). Common keys: hide_mail (1 or 0), " +
		"time_zone (a Rails time zone name such as Berlin or UTC, empty for the server default), " +
		"comments_sorting (asc or desc), no_self_notified (1 or 0), warn_on_leaving_unsaved (1 or 0). " +
		"Other keys depend on the Redmine version; Redmine ignores keys it does not know, and list-valued keys are not supported " +
		"(write-only: no API response returns it)"
	languageFlagUsage         = "Interface language code such as en, de or pt-BR, empty for the server default (write-only: no API response returns it)"
	notifiedProjectFlagUsage  = "Project (identifier, name, or ID) to get all notifications for with --mail-notification selected; only projects with a membership count (repeatable, replaces the current set, \"\" clears it; write-only: no API response returns it)"
	mailNotificationFlagUsage = "Email notification preference (all, selected, only_my_events, only_assigned, only_owner, only_my_watches, none; only_my_watches needs Redmine 7.0+)"
	notifiedProjectsHint      = "; selected notifies about the --notified-project set"
)

// AddPrefFlag registers --pref on cmd.
func AddPrefFlag(cmd *cobra.Command, raws *[]string) {
	cmd.Flags().StringArrayVar(raws, "pref", nil, prefFlagUsage)
}

// AddLanguageFlag registers --language on cmd.
func AddLanguageFlag(cmd *cobra.Command, language *string) {
	cmd.Flags().StringVar(language, "language", "", languageFlagUsage)
}

// AddMailNotificationFlag registers --mail-notification with completion.
// withNotifiedProjects mentions --notified-project, for commands that have it.
func AddMailNotificationFlag(cmd *cobra.Command, value *string, withNotifiedProjects bool) {
	usage := mailNotificationFlagUsage
	if withNotifiedProjects {
		usage += notifiedProjectsHint
	}
	cmd.Flags().StringVar(value, "mail-notification", "", usage)
	_ = cmd.RegisterFlagCompletionFunc("mail-notification", CompleteMailNotification)
}

// AddNotifiedProjectFlag registers --notified-project with project
// completion.
func AddNotifiedProjectFlag(cmd *cobra.Command, f *cmdutil.Factory, values *[]string) {
	cmd.Flags().StringArrayVar(values, "notified-project", nil, notifiedProjectFlagUsage)
	_ = cmd.RegisterFlagCompletionFunc("notified-project", cmdutil.CompleteProjects(f))
}

// ParsePrefs turns --pref key=value entries into preferences. Entries
// without "=" or with an empty key are rejected; keys are not checked against
// a list, because the set of preferences differs between Redmine versions.
// A repeated key keeps the last value.
func ParsePrefs(raws []string) (models.UserPreferences, error) {
	if len(raws) == 0 {
		return nil, nil
	}
	pairs := make(map[string]string, len(raws))
	for _, raw := range raws {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--pref %q must be key=value", raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--pref %q: empty key", raw)
		}
		pairs[key] = value
	}
	for key, value := range pairs {
		if strings.ContainsAny(key, " \t[]") {
			return nil, fmt.Errorf("--pref key %q is not a preference name (use key=value, e.g. hide_mail=1)", key)
		}
		// Redmine stores the text as given, and most boolean preferences
		// only read "1" as on, so "true" would silently mean off.
		if v := strings.ToLower(strings.TrimSpace(value)); v == "true" || v == "false" {
			hint := "0"
			if v == "true" {
				hint = "1"
			}
			return nil, fmt.Errorf("--pref %s=%s: use %s=%s; Redmine preferences take 1 or 0, and it would store %q as text", key, value, key, hint, value)
		}
	}
	return models.UserPreferences(pairs), nil
}

// ResolveNotifiedProjects resolves --notified-project values to project IDs.
// Numeric IDs are used as given; identifiers and names are looked up. Blank
// values are skipped, so a lone "" yields an empty, non-nil set that clears
// the user's notified projects.
func ResolveNotifiedProjects(ctx context.Context, client *api.Client, inputs []string) ([]int, error) {
	ids := []int{}
	for _, in := range inputs {
		in = strings.TrimSpace(in)
		if in == "" {
			continue
		}
		if id, err := strconv.Atoi(in); err == nil {
			ids = append(ids, id)
			continue
		}
		id, _, err := resolver.ResolveProject(ctx, client, in)
		if err != nil {
			return nil, fmt.Errorf("resolve --notified-project: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
