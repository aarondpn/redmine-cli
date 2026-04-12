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

	pc, err := config.LoadProfiles(configPath, debug.New(nil))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	printer := f.Printer("")

	// Honor --profile override, falling back to active profile
	profileName := f.ProfileOverride
	if profileName == "" {
		profileName = pc.ActiveProfile
	}

	// Fall back to sole profile if no active profile set (matching Load behavior)
	if profileName == "" && len(pc.Profiles) == 1 {
		for profileName = range pc.Profiles {
			// take the only key
		}
	}

	if len(pc.Profiles) == 0 || profileName == "" {
		printer.Warning("No active profile. Run 'redmine auth login' to set up.")
		return nil
	}

	profile, ok := pc.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found in config", profileName)
	}

	kvs := []output.KeyValue{
		{Key: "Profile", Value: profileName},
		{Key: "Server", Value: profile.Server},
		{Key: "Auth Method", Value: profile.AuthMethod},
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

	if profile.DefaultProject != "" {
		kvs = append(kvs, output.KeyValue{Key: "Default Project", Value: profile.DefaultProject})
	}

	printer.Detail(kvs)
	return nil
}
