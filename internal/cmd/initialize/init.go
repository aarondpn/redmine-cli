package initialize

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
)

// NewCmdInit creates the init command for first-time setup.
func NewCmdInit(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up Redmine CLI configuration",
		Long:  "Interactive setup wizard to configure your Redmine server connection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(f)
		},
	}
	return cmd
}

func runInit(f *cmdutil.Factory) error {
	var (
		server     string
		authMethod string
		apiKey     string
		username   string
		password   string
		defProject string
	)

	// Step 1: Server URL and auth method
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Redmine Server URL").
				Description("e.g., https://redmine.example.com").
				Value(&server).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("server URL is required")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Authentication Method").
				Options(
					huh.NewOption("API Key (recommended)", "apikey"),
					huh.NewOption("Username & Password", "basic"),
				).
				Value(&authMethod),
		),
	).Run()
	if err != nil {
		return err
	}

	// Step 2: Credentials
	if authMethod == "apikey" {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API Key").
					Description("Found at My Account > API access key").
					Value(&apiKey).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("API key is required")
						}
						return nil
					}),
			),
		).Run()
	} else {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Username").
					Value(&username),
				huh.NewInput().
					Title("Password").
					EchoMode(huh.EchoModePassword).
					Value(&password),
			),
		).Run()
	}
	if err != nil {
		return err
	}

	// Step 3: Test connection
	cfg := &config.Config{
		Server:     server,
		AuthMethod: authMethod,
		APIKey:     apiKey,
		Username:   username,
		Password:   password,
		PageSize:   25,
	}

	printer := f.Printer("")
	stop := printer.Spinner("Testing connection...")

	client, err := api.NewClient(cfg)
	if err != nil {
		stop()
		return fmt.Errorf("failed to create client: %w", err)
	}

	user, err := client.Users.Current(context.Background())
	stop()
	if err != nil {
		printer.Error("Connection failed: " + cmdutil.FormatError(err))
		return fmt.Errorf("could not connect to Redmine server")
	}

	printer.Success(fmt.Sprintf("Connected as %s %s (%s)", user.FirstName, user.LastName, user.Login))

	// Step 4: Default project (optional)
	stop = printer.Spinner("Fetching projects...")
	projects, _, err := client.Projects.List(context.Background(), nil, 100)
	stop()

	if err == nil && len(projects) > 0 {
		options := []huh.Option[string]{
			huh.NewOption("(none)", ""),
		}
		for _, p := range projects {
			options = append(options, huh.NewOption(p.Name, p.Identifier))
		}

		huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Default Project (optional)").
					Description("Used when --project is not specified").
					Options(options...).
					Value(&defProject),
			),
		).Run()
	}

	cfg.DefaultProject = defProject
	cfg.OutputFormat = "table"

	// Step 5: Save config
	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}

	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	printer.Success(fmt.Sprintf("Configuration saved to %s", configPath))

	// Step 6: Offer agent skill installation
	if _, err := exec.LookPath("npx"); err == nil {
		var installSkill bool
		huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Install AI agent skill?").
					Description("Installs a skill that teaches AI coding agents (Claude Code, Cursor, etc.) how to use redmine-cli effectively.").
					Value(&installSkill),
			),
		).Run()

		if installSkill {
			stop := printer.Spinner("Installing agent skill...")
			out, err := exec.Command("npx", "-y", "skills", "add", "aarondpn/redmine-cli", "--skill", "redmine-cli", "-g", "-y").CombinedOutput()
			stop()
			if err != nil {
				printer.Warning(fmt.Sprintf("Could not install agent skill: %s\n%s", err, string(out)))
				printer.Warning("You can install it manually: npx skills add aarondpn/redmine-cli --skill redmine-cli -g")
			} else {
				printer.Success("Agent skill installed globally. AI agents will now know how to use redmine-cli.")
			}
		}
	}

	return nil
}
