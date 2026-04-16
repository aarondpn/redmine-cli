package auth

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/debug"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdList creates the auth list command.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List authentication profiles",
		Long:  "Show all configured profiles with their server URLs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(f)
		},
	}

	return cmd
}

func runList(f *cmdutil.Factory) error {
	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}

	pc, err := config.LoadProfiles(configPath, debug.New(nil))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	printer := f.Printer("")

	// Sort profile names for stable output
	names := make([]string, 0, len(pc.Profiles))
	for name := range pc.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	activeProfile := config.EffectiveProfileName(pc, f.ProfileOverride)

	if printer.Format() == output.FormatJSON {
		type profileEntry struct {
			Name   string `json:"name"`
			Server string `json:"server"`
			Active bool   `json:"active"`
		}
		profiles := make([]profileEntry, 0, len(names))
		for _, name := range names {
			p := pc.Profiles[name]
			profiles = append(profiles, profileEntry{
				Name:   name,
				Server: p.Server,
				Active: name == activeProfile,
			})
		}
		printer.JSON(profiles)
		return nil
	}

	if len(pc.Profiles) == 0 {
		printer.Warning(noProfilesConfiguredMessage)
		return nil
	}

	var kvs []output.KeyValue
	for _, name := range names {
		p := pc.Profiles[name]
		marker := "  "
		if name == activeProfile {
			marker = "* "
		}
		label := marker + name
		kvs = append(kvs, output.KeyValue{Key: label, Value: p.Server})
	}

	printer.Detail(kvs)
	return nil
}
