package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// NewCmdStatus creates the auth status command.
func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Args:  cobra.NoArgs,
		Short: "Show current authentication status",
		Long:  "Display the active profile, server, and authenticated user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmdutil.Context(cmd), f)
		},
	}

	return cmd
}

func runStatus(ctx context.Context, f *cmdutil.Factory) error {
	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}

	cfg, err := f.Config()
	if err != nil {
		if config.IsNoActiveProfileError(err) {
			printer := f.Printer("")
			if printer.Format() == output.FormatJSON {
				printer.JSON(map[string]any{
					"active":  false,
					"reason":  "no_active_profile",
					"message": noActiveProfileMessage,
				})
				return nil
			}
			printer.Warning(noActiveProfileMessage)
			return nil
		}
		return err
	}

	pc, err := config.LoadProfiles(configPath, debug.New(nil))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	printer := f.Printer("")

	profileName := config.EffectiveProfileName(pc, f.ProfileOverride)
	if profileName == "" && cfg.Server == "" {
		reason := "no_active_profile"
		message := noActiveProfileMessage
		if len(pc.Profiles) == 0 {
			reason = "no_profiles_configured"
			message = noProfilesConfiguredMessage
		}
		if printer.Format() == output.FormatJSON {
			printer.JSON(map[string]any{
				"active":  false,
				"reason":  reason,
				"message": message,
			})
			return nil
		}
		printer.Warning(message)
		return nil
	}

	if profileName == "" {
		profileName = "(override)"
	} else if _, ok := pc.Profiles[profileName]; !ok {
		return profileNotFoundError(profileName)
	}

	kvs := []output.KeyValue{
		{Key: "Profile", Value: profileName},
		{Key: "Server", Value: cfg.Server},
		{Key: "Auth Method", Value: cfg.AuthMethod},
	}
	authOK := true
	var failure *statusFailure

	// Try to fetch current user
	client, err := f.ApiClient()
	if err != nil {
		authOK = false
		kvs = append(kvs, output.KeyValue{Key: "User", Value: fmt.Sprintf("unavailable: %s", err)})
	} else {
		user, err := ops.GetCurrentUser(ctx, client, ops.GetCurrentUserInput{})
		if err == nil {
			kvs = append(kvs, output.KeyValue{Key: "User", Value: fmt.Sprintf("%s %s (%s)", user.FirstName, user.LastName, user.Login)})
		} else {
			authOK = false
			failure = classifyStatusError(err)
			kvs = append(kvs, output.KeyValue{Key: "User", Value: failure.user})
			kvs = append(kvs, output.KeyValue{Key: "Reason", Value: failure.reason})
			kvs = append(kvs, output.KeyValue{Key: "Hint", Value: failure.hint})
		}
	}

	if cfg.DefaultProject != "" {
		kvs = append(kvs, output.KeyValue{Key: "Default Project", Value: cfg.DefaultProject})
	}

	if printer.Format() == output.FormatJSON {
		payload := make(map[string]any, len(kvs)+1)
		payload["active"] = authOK
		for _, kv := range kvs {
			payload[jsonKey(kv.Key)] = kv.Value
		}
		if failure != nil {
			payload["message"] = failure.message
			if failure.anonymous {
				payload["anonymous"] = true
			}
		}
		printer.JSON(payload)
		return nil
	}

	printer.Detail(kvs)
	if failure != nil {
		printer.Warning(failure.message)
	}
	return nil
}

// statusFailure describes why the current-user probe failed. reason is a
// stable machine-readable code (a JSON field); message and hint are prose.
type statusFailure struct {
	user    string
	reason  string
	message string
	hint    string
	// anonymous is set when Redmine answered the probe as it does for a
	// request without a login, so the configured credentials are not in
	// effect and requests run as anonymous.
	anonymous bool
}

// Stable values of the "reason" field when the current-user probe fails.
const (
	reasonCredentialsRejected = "credentials_rejected"
	reasonRESTAPIDisabled     = "rest_api_disabled"
	reasonRequestFailed       = "request_failed"
)

// classifyStatusError separates "Redmine answered and rejected us" from "we
// could not talk to Redmine". /users/current.json needs a login but no role
// permission, so against a reachable Redmine only these outcomes exist:
//
//   - 401: the key was not recognised and Redmine fell back to anonymous.
//     Public endpoints keep answering with public data, which is why list
//     commands can succeed with a bogus key.
//   - 403: Redmine answers every login-requiring request with 403 while the
//     REST API is disabled, whatever the key. A proxy can do the same.
func classifyStatusError(err error) *statusFailure {
	var apiErr *api.APIError
	switch {
	case errors.As(err, &apiErr) && apiErr.IsAuthError():
		return &statusFailure{
			user:      "authentication failed",
			reason:    reasonCredentialsRejected,
			message:   "Redmine rejected the configured credentials and treats requests as anonymous; commands may return only public data or fail.",
			hint:      "Check the API key (My account > API access key) and run 'redmine auth login' to configure a valid one.",
			anonymous: true,
		}
	case errors.As(err, &apiErr) && apiErr.IsForbidden():
		return &statusFailure{
			user:      "authentication failed",
			reason:    reasonRESTAPIDisabled,
			message:   "Redmine refused the credentials check with 403, which it does for every login-requiring request while the REST API is disabled (or when a proxy blocks the request).",
			hint:      "Ask a Redmine administrator to enable Administration > Settings > API > Enable REST web service, or check any proxy or firewall in front of Redmine.",
			anonymous: true,
		}
	default:
		return &statusFailure{
			user:    fmt.Sprintf("unavailable: %s", err),
			reason:  reasonRequestFailed,
			message: "Could not verify the credentials; the request to Redmine failed.",
			hint:    "Check the server URL and your network connection, then run 'redmine auth status' again.",
		}
	}
}

// jsonKey converts a human label ("Default Project") into a stable snake_case
// JSON field name ("default_project").
func jsonKey(label string) string {
	out := make([]byte, 0, len(label))
	for i := 0; i < len(label); i++ {
		c := label[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c-'A'+'a')
		case c == ' ':
			out = append(out, '_')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
