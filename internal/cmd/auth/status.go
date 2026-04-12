package auth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/debug"
	"github.com/aarondpn/redmine-cli/internal/output"
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
		if len(pc.Profiles) == 0 {
			printer.Warning(noProfilesConfiguredMessage)
		} else {
			printer.Warning(noActiveProfileMessage)
		}
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

	// Try to fetch current user
	client, err := f.ApiClient()
	if err == nil {
		user, err := client.Users.Current(context.Background())
		if err == nil {
			kvs = append(kvs, output.KeyValue{Key: "User", Value: fmt.Sprintf("%s %s (%s)", user.FirstName, user.LastName, user.Login)})
		} else {
			kvs = append(kvs, output.KeyValue{Key: "User", Value: "authentication failed"})
		}
	}

	if cfg.DefaultProject != "" {
		kvs = append(kvs, output.KeyValue{Key: "Default Project", Value: cfg.DefaultProject})
	}

	printer.Detail(kvs)
	return nil
}
