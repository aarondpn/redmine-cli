package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	apicmd "github.com/aarondpn/redmine-cli/v2/internal/cmd/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/attachment"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/auth"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/category"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/completion"
	customfield "github.com/aarondpn/redmine-cli/v2/internal/cmd/custom_field"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/enumeration"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/files"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/group"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/installskill"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/issue"
	mcpcmd "github.com/aarondpn/redmine-cli/v2/internal/cmd/mcp"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/membership"
	myaccount "github.com/aarondpn/redmine-cli/v2/internal/cmd/my_account"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/project"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/query"
	"github.com/aarondpn/redmine-cli/v2/internal/cmd/role"
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
		apiKeyFile   string
		profile      string
		noColor      bool
		verbose      bool
		cfgFile      string
		outputFormat string
		readOnly     bool
		headers      []string
		jqExpr       string
	)

	cmd := &cobra.Command{
		Use:   "redmine",
		Short: "CLI tool for the Redmine project management API",
		Long:  "A command-line interface for interacting with Redmine's REST API.",
		Example: `  # List open issues in a project
  redmine issues list --project myproject

  # Print the IDs of matching issues, one per line
  redmine issues list --project myproject --jq '.[].id'

  # Print a single field of an issue
  redmine issues get 123 --jq '.status.name'

  # Capture the ID of a newly created issue
  redmine issues create --project myproject --tracker Bug --subject "Crash on save" --jq '.id'`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.ApplyGlobalFlags(f, cmd); err != nil {
				return err
			}
			if err := cmdutil.CheckOutputFlag(cmd); err != nil {
				return err
			}
			return applyJQFlag(cmd, f, jqExpr, cmd.Flags().Changed("jq"))
		},
		// Printer methods have no error return, so a --jq runtime failure
		// is recorded on the factory and surfaced once the command is done.
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return f.JQError()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	cmd.PersistentFlags().StringVarP(&server, "server", "s", "", "Redmine server URL")
	cmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication (visible in process listings; prefer --api-key-file, REDMINE_API_KEY, or a keyring profile)")
	cmd.PersistentFlags().StringVar(&apiKeyFile, "api-key-file", "", `Read the API key from a file ("-" for stdin) instead of passing it on the command line`)
	cmd.MarkFlagsMutuallyExclusive("api-key", "api-key-file")
	cmd.PersistentFlags().StringVar(&profile, "profile", "", "Use a specific auth profile")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file path (default ~/.redmine-cli.yaml)")
	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format: table, json")
	cmd.PersistentFlags().BoolVar(&readOnly, "read-only", false, "Refuse all requests that modify data on the server")
	cmd.PersistentFlags().StringArrayVar(&headers, "header", nil, `Extra HTTP header sent to the server, as "Name: value" (repeatable)`)
	cmd.PersistentFlags().StringVarP(&jqExpr, "jq", "q", "", "Filter JSON output using a jq expression (implies --output json)")
	_ = cmd.RegisterFlagCompletionFunc("output", cmdutil.CompleteOutputFormat)
	_ = cmd.RegisterFlagCompletionFunc("jq", cobra.NoFileCompletions)

	// Version
	cmd.Version = version
	api.UserAgent = "redmine-cli/" + version

	// Subcommands
	cmd.AddCommand(apicmd.NewCmdAPI(f))
	cmd.AddCommand(auth.NewCmdAuth(f))
	cmd.AddCommand(issue.NewCmdIssue(f))
	cmd.AddCommand(attachment.NewCmdAttachments(f))
	cmd.AddCommand(group.NewCmdGroup(f))
	cmd.AddCommand(membership.NewCmdMemberships(f))
	cmd.AddCommand(project.NewCmdProject(f))
	cmd.AddCommand(query.NewCmdQueries(f))
	cmd.AddCommand(role.NewCmdRoles(f))
	cmd.AddCommand(timecmd.NewCmdTime(f))
	cmd.AddCommand(user.NewCmdUser(f))
	cmd.AddCommand(myaccount.NewCmdMyAccount(f))
	cmd.AddCommand(tracker.NewCmdTrackers(f))
	cmd.AddCommand(customfield.NewCmdCustomFields(f))
	cmd.AddCommand(category.NewCmdCategories(f))
	cmd.AddCommand(status.NewCmdStatuses(f))
	cmd.AddCommand(enumeration.NewCmdPriorities(f))
	cmd.AddCommand(enumeration.NewCmdActivities(f))
	cmd.AddCommand(enumeration.NewCmdDocumentCategories(f))
	cmd.AddCommand(versioncmd.NewCmdVersions(f))
	cmd.AddCommand(files.NewCmdFiles(f))
	cmd.AddCommand(search.NewCmdSearch(f))
	cmd.AddCommand(wiki.NewCmdWiki(f))
	cmd.AddCommand(mcpcmd.NewCmdMCP(f))
	cmd.AddCommand(completion.NewCmdCompletion())
	cmd.AddCommand(installskill.NewCmdInstallSkill(f))
	cmd.AddCommand(update.NewCmdUpdate(f, version))
	cmd.AddCommand(newCmdConfig(f))

	return cmd, f
}

// applyJQFlag validates --jq and switches the command to JSON output. The
// expression is compiled here so a typo fails before any API request.
func applyJQFlag(cmd *cobra.Command, f *cmdutil.Factory, expr string, set bool) error {
	if !set {
		return nil
	}
	if cmdutil.HasNoJSONOutput(cmd) {
		return fmt.Errorf("--jq is not supported for %s: it does not print JSON", cmd.CommandPath())
	}
	// cmd.Flags() also sees a leaf command's local --output, which shadows
	// the persistent one.
	if of := cmd.Flags().Lookup("output"); of != nil && of.Changed && of.Value.String() != output.FormatJSON {
		f.OutputFormat = of.Value.String()
		return fmt.Errorf("--jq requires JSON output and cannot be combined with --output %s", of.Value.String())
	}
	f.OutputFormat = output.FormatJSON
	q, err := output.CompileJQ(expr)
	if err != nil {
		return err
	}
	f.JQ = q
	return nil
}

func newCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Args:  cobra.NoArgs,
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

			// Header values may carry proxy tokens, so only names are shown.
			headerNames := make([]string, 0, len(cfg.Headers))
			for name := range cfg.Headers {
				headerNames = append(headerNames, name)
			}
			sort.Strings(headerNames)

			printer := f.Printer("")
			if printer.Format() == output.FormatJSON {
				printer.JSON(map[string]string{
					"profile":         profileName,
					"server":          cfg.Server,
					"auth_method":     cfg.AuthMethod,
					"default_project": cfg.DefaultProject,
					"output_format":   cfg.OutputFormat,
					"read_only":       strconv.FormatBool(cfg.ReadOnly),
					"headers":         strings.Join(headerNames, ", "),
				})
				return nil
			}
			printer.Detail([]output.KeyValue{
				{Key: "Profile", Value: profileName},
				{Key: "Server", Value: cfg.Server},
				{Key: "Auth Method", Value: cfg.AuthMethod},
				{Key: "Default Project", Value: cfg.DefaultProject},
				{Key: "Output Format", Value: cfg.OutputFormat},
				{Key: "Read Only", Value: strconv.FormatBool(cfg.ReadOnly)},
				{Key: "Headers", Value: strings.Join(headerNames, ", ")},
			})
			return nil
		},
	}
	return cmd
}
