package auth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// NewCmdStatus creates the auth status command.
func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		Long:  "Display the active profile, server, and authenticated user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(f)
		},
	}

	return cmd
}

func runStatus(f *cmdutil.Factory) error {
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

	// Try to fetch current user
	client, err := f.ApiClient()
	if err != nil {
		authOK = false
		kvs = append(kvs, output.KeyValue{Key: "User", Value: fmt.Sprintf("unavailable: %s", err)})
	} else {
		user, err := client.Users.Current(context.Background())
		if err == nil {
			kvs = append(kvs, output.KeyValue{Key: "User", Value: fmt.Sprintf("%s %s (%s)", user.FirstName, user.LastName, user.Login)})
		} else {
			authOK = false
			kvs = append(kvs, output.KeyValue{Key: "User", Value: "authentication failed"})
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
		printer.JSON(payload)
		return nil
	}

	printer.Detail(kvs)
	return nil
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
