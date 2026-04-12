package auth

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/debug"
)

// NewCmdLogout creates the auth logout command.
func NewCmdLogout(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [profile]",
		Short: "Remove a profile",
		Long:  "Remove the specified profile (or the active profile if none specified).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		name = pc.ActiveProfile
	}

	if name == "" {
		return fmt.Errorf("no profile specified and no active profile set")
	}

	if _, ok := pc.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
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

	if !confirm {
		return nil
	}

	printer := f.Printer("")

	if err := config.DeleteProfile(name, configPath); err != nil {
		return fmt.Errorf("removing profile: %w", err)
	}

	printer.Success(fmt.Sprintf("Profile %q removed", name))
	return nil
}
