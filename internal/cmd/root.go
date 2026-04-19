package cmd

import (
	"os"

	"github.com/spf13/cobra"

	apicmd "github.com/aarondpn/redmine-cli/v2/internal/cmd/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/auth"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/category"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/completion"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/group"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/installskill"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/issue"
	mcpcmd "github.com/aarondpn/redmine-cli/v2/internal/cmd/mcp"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/membership"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/project"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/search"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/status"
	timecmd "github.com/aarondpn/redmine-cli/v2/internal/cmd/time"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/tracker"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/update"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/user"
	versioncmd "github.com/aarondpn/redmine-cli/v2/internal/cmd/version"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/wiki"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// NewRootCmd creates the root command.
func NewRootCmd(version string) *cobra.Command {
	cmd, _ := NewRootCmdWithFactory(version)
	return cmd
}

// NewRootCmdWithFactory creates the root command and returns the shared
// Factory so callers (e.g. main.go) can inspect runtime-resolved state like
// the selected output format after execution.
func NewRootCmdWithFactory(version string) (*cobra.Command, *cmdutil.Factory) {
	f := cmdutil.NewFactory()

	var (
		server       string
		apiKey       string
		profile      string
		noColor      bool
		verbose      bool
		cfgFile      string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "redmine",
		Short: "CLI tool for the Redmine project management API",
		Long:  "A command-line interface for interacting with Redmine's REST API.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			f.ConfigPath = cfgFile
			f.ProfileOverride = profile
			f.ServerOverride = server
			f.APIKeyOverride = apiKey
			if noColor {
				f.NoColorOverride = true
				os.Setenv("NO_COLOR", "1")
			}
			f.Verbose = verbose
			f.OutputFormat = outputFormat
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	cmd.PersistentFlags().StringVarP(&server, "server", "s", "", "Redmine server URL")
	cmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")
	cmd.PersistentFlags().StringVar(&profile, "profile", "", "Use a specific auth profile")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file path (default ~/.redmine-cli.yaml)")
	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format: table, json, csv")
	_ = cmd.RegisterFlagCompletionFunc("output", cmdutil.CompleteOutputFormat)

	// Version
	cmd.Version = version

	// Subcommands
	cmd.AddCommand(apicmd.NewCmdAPI(f))
	cmd.AddCommand(auth.NewCmdAuth(f))
	cmd.AddCommand(issue.NewCmdIssue(f))
	cmd.AddCommand(group.NewCmdGroup(f))
	cmd.AddCommand(membership.NewCmdMemberships(f))
	cmd.AddCommand(project.NewCmdProject(f))
	cmd.AddCommand(timecmd.NewCmdTime(f))
	cmd.AddCommand(user.NewCmdUser(f))
	cmd.AddCommand(tracker.NewCmdTrackers(f))
	cmd.AddCommand(category.NewCmdCategories(f))
	cmd.AddCommand(status.NewCmdStatuses(f))
	cmd.AddCommand(versioncmd.NewCmdVersions(f))
	cmd.AddCommand(search.NewCmdSearch(f))
	cmd.AddCommand(wiki.NewCmdWiki(f))
	cmd.AddCommand(mcpcmd.NewCmdMCP(f))
	cmd.AddCommand(completion.NewCmdCompletion())
	cmd.AddCommand(installskill.NewCmdInstallSkill(f))
	cmd.AddCommand(update.NewCmdUpdate(f, version))
	cmd.AddCommand(newCmdConfig(f))

	return cmd, f
}

func newCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Display current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}

			profileName := f.ProfileOverride
			configPath := f.ConfigPath
			if configPath == "" {
				configPath = config.DefaultConfigPath()
			}
			if pc, err := config.LoadProfiles(configPath, debug.New(nil)); err == nil {
				profileName = config.EffectiveProfileName(pc, f.ProfileOverride)
			}

			printer := f.Printer("")
			if printer.Format() == output.FormatJSON {
				printer.JSON(map[string]string{
					"profile":         profileName,
					"server":          cfg.Server,
					"auth_method":     cfg.AuthMethod,
					"default_project": cfg.DefaultProject,
					"output_format":   cfg.OutputFormat,
				})
				return nil
			}
			printer.Detail([]output.KeyValue{
				{Key: "Profile", Value: profileName},
				{Key: "Server", Value: cfg.Server},
				{Key: "Auth Method", Value: cfg.AuthMethod},
				{Key: "Default Project", Value: cfg.DefaultProject},
				{Key: "Output Format", Value: cfg.OutputFormat},
			})
			return nil
		},
	}
	return cmd
}
