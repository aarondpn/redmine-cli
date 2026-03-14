package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aarondpn/redmine-cli/internal/cmd/completion"
	initcmd "github.com/aarondpn/redmine-cli/internal/cmd/initialize"
	"github.com/aarondpn/redmine-cli/internal/cmd/issue"
	"github.com/aarondpn/redmine-cli/internal/cmd/project"
	"github.com/aarondpn/redmine-cli/internal/cmd/status"
	timecmd "github.com/aarondpn/redmine-cli/internal/cmd/time"
	"github.com/aarondpn/redmine-cli/internal/cmd/tracker"
	"github.com/aarondpn/redmine-cli/internal/cmd/update"
	"github.com/aarondpn/redmine-cli/internal/cmd/user"
	versioncmd "github.com/aarondpn/redmine-cli/internal/cmd/version"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewRootCmd creates the root command.
func NewRootCmd(version string) *cobra.Command {
	f := cmdutil.NewFactory()

	var (
		server    string
		apiKey    string
		defProject string
		outputFmt string
		noColor   bool
		verbose   bool
		cfgFile   string
	)

	cmd := &cobra.Command{
		Use:   "redmine",
		Short: "CLI tool for the Redmine project management API",
		Long:  "A command-line interface for interacting with Redmine's REST API.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			f.ConfigPath = cfgFile

			if server != "" {
				viper.Set("server", server)
			}
			if apiKey != "" {
				viper.Set("api_key", apiKey)
			}
			if noColor {
				viper.Set("no_color", true)
			}
			_ = verbose
			_ = defProject
			_ = outputFmt
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	cmd.PersistentFlags().StringVarP(&server, "server", "s", "", "Redmine server URL")
	cmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")
	cmd.PersistentFlags().StringVarP(&defProject, "project", "p", "", "Default project identifier")
	cmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "", "Output format: table, wide, json, csv")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file path (default ~/.redmine-cli.yaml)")

	// Version
	cmd.Version = version

	// Subcommands
	cmd.AddCommand(initcmd.NewCmdInit(f))
	cmd.AddCommand(issue.NewCmdIssue(f))
	cmd.AddCommand(project.NewCmdProject(f))
	cmd.AddCommand(timecmd.NewCmdTime(f))
	cmd.AddCommand(user.NewCmdUser(f))
	cmd.AddCommand(tracker.NewCmdTrackers(f))
	cmd.AddCommand(status.NewCmdStatuses(f))
	cmd.AddCommand(versioncmd.NewCmdVersions(f))
	cmd.AddCommand(completion.NewCmdCompletion())
	cmd.AddCommand(update.NewCmdUpdate(version))
	cmd.AddCommand(newCmdConfig(f))

	return cmd
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
			printer := f.Printer("")
			printer.Detail([]output.KeyValue{
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
