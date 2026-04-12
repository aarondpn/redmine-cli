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

	if len(pc.Profiles) == 0 {
		printer := f.Printer("")
		printer.Warning("No profiles configured. Run 'redmine auth login' to add one.")
		return nil
	}

	// Sort profile names for stable output
	names := make([]string, 0, len(pc.Profiles))
	for name := range pc.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	printer := f.Printer("")

	var kvs []output.KeyValue
	for _, name := range names {
		p := pc.Profiles[name]
		marker := "  "
		if name == pc.ActiveProfile {
			marker = "* "
		}
		label := marker + name
		kvs = append(kvs, output.KeyValue{Key: label, Value: p.Server})
	}

	printer.Detail(kvs)
	return nil
}
