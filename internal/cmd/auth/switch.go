package auth

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/debug"
)

// NewCmdSwitch creates the auth switch command.
func NewCmdSwitch(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch [profile]",
		Short: "Switch the active profile",
		Long:  "Set which profile to use by default. Shows an interactive selector if no name given.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSwitch(f, args)
		},
	}

	return cmd
}

func runSwitch(f *cmdutil.Factory, args []string) error {
	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}

	pc, err := config.LoadProfiles(configPath, debug.New(nil))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(pc.Profiles) == 0 {
		printer := f.Printer("")
		printer.Warning(noProfilesConfiguredMessage)
		return nil
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		// Interactive selection
		names := make([]string, 0, len(pc.Profiles))
		for n := range pc.Profiles {
			names = append(names, n)
		}
		sort.Strings(names)

		options := make([]huh.Option[string], 0, len(names))
		for _, n := range names {
			label := n
			if n == pc.ActiveProfile {
				label += " (active)"
			}
			p := pc.Profiles[n]
			label += " — " + p.Server
			options = append(options, huh.NewOption(label, n))
		}

		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select profile").
					Options(options...).
					Value(&name),
			),
		).Run()
		if err != nil {
			return err
		}
	}

	if err := config.SetActiveProfile(name, configPath); err != nil {
		return err
	}

	printer := f.Printer("")
	printer.Success(fmt.Sprintf("Switched to profile %q (%s)", name, pc.Profiles[name].Server))
	return nil
}
