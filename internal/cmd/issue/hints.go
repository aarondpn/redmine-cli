package issue

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// issueWritePath matches the endpoints Redmine uses for issue writes:
// POST /issues.json and PUT /issues/<id>.json. Other 422s raised while a
// command runs (e.g. an oversized upload to /uploads.json) must not get the
// issue-field hint.
var issueWritePath = regexp.MustCompile(`/issues(/\d+)?\.json$`)

// issuePath matches a single-issue endpoint, used to scope the not-found and
// forbidden hints to the issue itself rather than e.g. a missing version.
var issuePath = regexp.MustCompile(`/issues/\d+\.json$`)

// addErrorHints decorates issue subcommands so API failures carry a short,
// actionable next step in both the human error text and the JSON error
// envelope (error.hint). Redmine's validation messages are localized, so the
// hints are generic per status code and never parse the message text.
//
// Redmine answers 404 only for issues that do not exist; an existing issue the
// user cannot see (private, or in a project they are not a member of) yields
// 403, as does a visible issue the user's role may not change.
func addErrorHints(f *cmdutil.Factory, parent *cobra.Command) {
	for _, c := range parent.Commands() {
		switch c.Name() {
		case "create":
			wrapRunEWithHint(c, func(cmd *cobra.Command, _ []string, apiErr *api.APIError) string {
				if apiErr.IsValidationError() && matchesPath(apiErr, issueWritePath) {
					return createValidationHint(createHintProject(f, cmd))
				}
				return ""
			})
		case "update", "assign", "close", "reopen", "comment", "delete":
			wrapRunEWithHint(c, func(_ *cobra.Command, args []string, apiErr *api.APIError) string {
				switch {
				case apiErr.IsValidationError() && matchesPath(apiErr, issueWritePath):
					return updateValidationHint(firstArg(args))
				case apiErr.IsNotFound() && matchesPath(apiErr, issuePath):
					return issueNotFoundHint(firstArg(args))
				case apiErr.IsForbidden() && matchesPath(apiErr, issuePath):
					return issueWriteForbiddenHint(firstArg(args))
				}
				return ""
			})
		case "get":
			wrapRunEWithHint(c, func(_ *cobra.Command, args []string, apiErr *api.APIError) string {
				switch {
				case apiErr.IsNotFound() && matchesPath(apiErr, issuePath):
					return issueNotFoundHint(firstArg(args))
				case apiErr.IsForbidden() && matchesPath(apiErr, issuePath):
					return issueReadForbiddenHint(firstArg(args))
				}
				return ""
			})
		}
	}
}

// wrapRunEWithHint wraps cmd.RunE so that API errors returned by it are
// decorated with the hint computed by hintFor. Non-API errors and errors for
// which hintFor returns "" pass through unchanged.
func wrapRunEWithHint(cmd *cobra.Command, hintFor func(cmd *cobra.Command, args []string, apiErr *api.APIError) string) {
	run := cmd.RunE
	if run == nil {
		return
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := run(cmd, args)
		if err == nil {
			return nil
		}
		var apiErr *api.APIError
		if !errors.As(err, &apiErr) {
			return err
		}
		return cmdutil.WithHint(err, hintFor(cmd, args, apiErr))
	}
}

func matchesPath(apiErr *api.APIError, re *regexp.Regexp) bool {
	u, err := url.Parse(apiErr.URL)
	if err != nil {
		return false
	}
	return re.MatchString(u.Path)
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return "ID"
	}
	return args[0]
}

// createHintProject returns the canonical identifier of the project the
// create command targeted (flag or configured default), so the suggested
// commands work even when the user passed a project name. Falls back to the
// raw input when resolution fails, or a placeholder when there is none.
func createHintProject(f *cmdutil.Factory, cmd *cobra.Command) string {
	var project string
	if fl := cmd.Flags().Lookup("project"); fl != nil {
		project = fl.Value.String()
	}
	project = cmdutil.DefaultProject(f, project)
	if project == "" {
		return "PROJECT"
	}
	if identifier, err := cmdutil.ResolveProjectIdentifier(cmdutil.Context(cmd), f, project); err == nil && identifier != "" {
		project = identifier
	}
	return shellQuote(project)
}

// shellQuote single-quotes s for POSIX shells when it contains anything
// beyond the characters Redmine allows in project identifiers.
func shellQuote(s string) string {
	safe := strings.IndexFunc(s, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return false
		}
		return true
	}) == -1
	if safe && s != "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func lookupValuesCommands(project string) string {
	return fmt.Sprintf("`redmine projects get %[1]s --include trackers,issue_categories,issue_custom_fields`, "+
		"`redmine versions list --project %[1]s --open`, and `redmine memberships list --project %[1]s` (assignable users)", project)
}

func createValidationHint(project string) string {
	return "Required fields and allowed values depend on the project and tracker. Look them up with " +
		lookupValuesCommands(project) + "."
}

func updateValidationHint(id string) string {
	return fmt.Sprintf("Required fields and allowed values depend on the issue's project, tracker, and workflow. "+
		"Get the project ID from `redmine issues get %s -o json` (field project.id), then look up values with ", id) +
		lookupValuesCommands("PROJECT") + "."
}

func issueNotFoundHint(id string) string {
	return fmt.Sprintf("Issue #%s does not exist on this server. To find an issue by text, search all projects with `redmine search KEYWORDS --issues --scope all`.", id)
}

func issueReadForbiddenHint(id string) string {
	return fmt.Sprintf("Issue #%s exists but is not visible to the current user: it is private or in a project they are not a member of. Don't retry with other credentials; ask the user or a project manager for access.", id)
}

func issueWriteForbiddenHint(id string) string {
	return fmt.Sprintf("Issue #%s is either not visible to the current user (private, or in a project they are not a member of) or their role may not make this change. Don't retry with other credentials; ask the user or a project manager.", id)
}
