package auth

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
)

// NewCmdLogout creates the auth logout command.
func NewCmdLogout(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [profile]",
		Short: "Remove a profile",
		Long:  "Remove the specified profile (or the active profile if none specified).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.PrepareInteractiveCommand(cmd, f); err != nil {
				return err
			}
			return runLogout(f, args)
		},
	}

	return cmd
}

func runLogout(f *cmdutil.Factory, args []string) error {
	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}

	pc, err := config.LoadProfiles(configPath, debug.New(nil))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	name, err := resolveLogoutProfileName(pc, args, f.ProfileOverride)
	if err != nil {
		if err.Error() == noProfilesConfiguredMessage {
			printer := f.Printer("")
			printer.Outcome(false, output.ActionLoggedOut, "profile", nil, noProfilesConfiguredMessage)
			return nil
		}
		return err
	}

	deleted, ok := pc.Profiles[name]
	if !ok {
		return profileNotFoundError(name)
	}

	// Confirm deletion
	var confirm bool
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Remove profile %q (%s)?", name, pc.Profiles[name].Server)).
				Value(&confirm),
		),
	).Run()
	if err != nil {
		return err
	}

	printer := f.Printer("")

	if !confirm {
		printer.Outcome(false, output.ActionLoggedOut, "profile", name, "Logout cancelled")
		return nil
	}

	if err := config.DeleteProfile(name, configPath); err != nil {
		return fmt.Errorf("removing profile: %w", err)
	}

	warnSkippedKeyringCleanup(printer, &deleted)

	printer.Action(output.ActionLoggedOut, "profile", name, fmt.Sprintf("Profile %q removed", name))
	return nil
}

// warnSkippedKeyringCleanup tells the user how to find the credential that
// REDMINE_NO_KEYRING left behind, since its config reference is now gone.
func warnSkippedKeyringCleanup(printer output.Printer, deleted *config.Config) {
	if deleted.CredentialStore == config.CredentialStoreKeyring && deleted.KeyringID != "" && secrets.Disabled() {
		printer.Warning(fmt.Sprintf("Keyring cleanup skipped because REDMINE_NO_KEYRING is set. The credential remains in the system keyring under service %q; remove it with your keyring manager", secrets.Service))
	}
}

func resolveLogoutProfileName(pc *config.ProfileConfig, args []string, override string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	name := config.EffectiveProfileName(pc, override)
	if name == "" {
		if pc != nil && len(pc.Profiles) == 0 {
			return "", errors.New(noProfilesConfiguredMessage)
		}
		return "", errors.New(noActiveProfileMessage)
	}

	return name, nil
}
