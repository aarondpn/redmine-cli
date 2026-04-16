package auth

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdLogin creates the auth login command.
func NewCmdLogin(f *cmdutil.Factory) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a Redmine instance",
		Long:  "Interactive setup to authenticate with a Redmine server and save the profile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.PrepareInteractiveCommand(cmd, f); err != nil {
				return err
			}
			return runLogin(f, name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Profile name (default: derived from server hostname)")

	return cmd
}

func runLogin(f *cmdutil.Factory, profileName string) error {
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

	// Derive default profile name from server URL
	defaultName := config.ProfileNameFromURL(server)
	if defaultName == "" {
		defaultName = "default"
	}

	// Step 2: Profile name (with default from hostname)
	if profileName == "" {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Profile name").
					Description("A short name for this connection").
					Placeholder(defaultName).
					Value(&profileName),
			),
		).Run()
		if err != nil {
			return err
		}
		if profileName == "" {
			profileName = defaultName
		}
	}

	// Step 3: Credentials
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

	// Step 4: Test connection
	cfg := &config.Config{
		Server:     server,
		AuthMethod: authMethod,
		APIKey:     apiKey,
		Username:   username,
		Password:   password,
	}

	printer := f.Printer("")
	stop := printer.Spinner("Testing connection...")

	client, err := api.NewClient(cfg, nil)
	if err != nil {
		stop()
		return fmt.Errorf("failed to create client: %w", err)
	}

	user, err := client.Users.Current(context.Background())
	stop()
	if err != nil {
		printer.Error("Connection failed: " + cmdutil.FormatError(err))
		return fmt.Errorf("could not connect to Redmine server: %w", err)
	}

	printer.Success(fmt.Sprintf("Connected as %s %s (%s)", user.FirstName, user.LastName, user.Login))

	// Step 5: Default project (optional)
	stop = printer.Spinner("Fetching projects...")
	projects, _, err := client.Projects.List(context.Background(), nil, 100, 0)
	stop()

	if err == nil && len(projects) > 0 {
		options := []huh.Option[string]{
			huh.NewOption("(none)", ""),
		}
		for _, p := range projects {
			options = append(options, huh.NewOption(p.Name, p.Identifier))
		}

		_ = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Default Project (optional)").
					Description("Used when --project is not specified").
					Options(options...).
					Value(&defProject).
					Height(5),
			),
		).Run()
	}

	cfg.DefaultProject = defProject
	cfg.OutputFormat = "table"

	// Step 6: Save profile
	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}

	if err := config.SaveProfile(profileName, cfg, configPath); err != nil {
		return fmt.Errorf("saving profile: %w", err)
	}

	// Set as active profile
	if err := config.SetActiveProfile(profileName, configPath); err != nil {
		return fmt.Errorf("setting active profile: %w", err)
	}

	printer.Action(output.ActionLoggedIn, "profile", profileName,
		fmt.Sprintf("Profile %q saved and activated (%s)", profileName, configPath))

	return nil
}
