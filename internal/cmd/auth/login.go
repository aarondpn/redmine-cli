package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
)

// loginOptions holds the login-specific flags.
type loginOptions struct {
	name           string
	useKeyring     bool
	keyringSet     bool
	defaultProject string

	oauth            bool
	clientID         string
	clientSecretFile string
	scopes           string
	noBrowser        bool

	// Resolved by RunE for runLogin.
	clientSecret  string
	relogin       bool
	reloginServer string
}

// loginNeedsTerminalError is returned when values are missing and there is no
// terminal to prompt on.
const loginNeedsTerminalError = `redmine auth login needs a terminal to prompt for missing values; pass them as flags instead, e.g.
  redmine auth login --server https://redmine.example.com --api-key-file -
  redmine auth login --server https://redmine.example.com --oauth --client-id ID --client-secret-file -
(the API key is read from standard input; --api-key-file also accepts a file path)`

// NewCmdLogin creates the auth login command.
func NewCmdLogin(f *cmdutil.Factory) *cobra.Command {
	var opts loginOptions

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.NoArgs,
		Short: "Log in to a Redmine instance",
		Long: `Authenticate with a Redmine server and save the profile.

In a terminal this starts an interactive wizard; values given as flags are not
asked for again. Without a terminal (scripts, CI, AI agents), pass the global
--server flag together with --api-key-file (or --api-key) and no prompt is
shown: the URL is normalized, the connection is tested, and the profile is
saved and activated. In that mode -o json prints the result.

A server URL without a scheme gets https://, and a URL pasted from the browser
(https://host/projects/foo/issues) is reduced to the Redmine base URL while a
sub-path such as /redmine is kept.

Headers passed with the global --header flag are used for the connection test
and saved to the profile, for servers behind a firewall or proxy that require them.

With --oauth you log in in the browser (Redmine 6.1 or newer). An administrator
registers the application under Administration > Applications with the redirect
URI http://127.0.0.1/callback. Running 'redmine auth login --oauth' again for an
OAuth profile reuses its client and logs in anew.`,
		Example: `  redmine auth login
  redmine auth login --name work
  redmine auth login --header "User-Agent: Mozilla/5.0"
  echo "$KEY" | redmine auth login --server https://redmine.example.com --api-key-file -
  redmine auth login --server redmine.example.com --api-key-file ~/.redmine-key --default-project web -o json
  echo "$SECRET" | redmine auth login --server https://redmine.example.com --client-id ID --client-secret-file -
  redmine auth login --oauth --no-browser --scopes default,admin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.oauth = opts.oauth || opts.clientID != "" || opts.clientSecretFile != "" || opts.scopes != "" || opts.noBrowser
			if opts.oauth && (f.APIKeyOverride != "" || f.APIKeyFile != "") {
				return errors.New("--oauth cannot be combined with --api-key or --api-key-file")
			}
			if opts.clientSecretFile != "" {
				secret, err := cmdutil.ReadSecretFile("--client-secret-file", opts.clientSecretFile, f.IOStreams.In)
				if err != nil {
					return err
				}
				opts.clientSecret = secret
			}
			nonInteractive := f.ServerOverride != "" && (f.APIKeyOverride != "" || f.APIKeyFile != "")
			if opts.oauth {
				nonInteractive = f.ServerOverride != "" && opts.clientID != "" && opts.clientSecret != ""
				if !nonInteractive {
					nonInteractive = applyRelogin(f, &opts)
				}
			}
			if !nonInteractive {
				if !hasTerminal(f) {
					return errors.New(loginNeedsTerminalError)
				}
				if err := cmdutil.PrepareInteractiveCommand(cmd, f); err != nil {
					return err
				}
			} else if f.OutputFormat == "" {
				// An active profile's output_format must not leak into the result.
				f.OutputFormat = output.FormatTable
			}
			opts.keyringSet = cmd.Flags().Changed("keyring")
			return runLogin(cmdutil.Context(cmd), f, opts, nonInteractive)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Profile name (default: derived from server hostname)")
	cmd.Flags().BoolVar(&opts.useKeyring, "keyring", false, "Store credentials in the system keyring instead of the plaintext config file")
	cmd.Flags().StringVar(&opts.defaultProject, "default-project", "", "Default project identifier to save in the profile")
	cmd.Flags().BoolVar(&opts.oauth, "oauth", false, "Log in with OAuth in the browser (Redmine 6.1+)")
	cmd.Flags().StringVar(&opts.clientID, "client-id", "", "OAuth application client ID (implies --oauth)")
	cmd.Flags().StringVar(&opts.clientSecretFile, "client-secret-file", "", `File with the OAuth client secret, or "-" for standard input (implies --oauth)`)
	cmd.Flags().StringVar(&opts.scopes, "scopes", "", `OAuth scopes, comma or space separated; "default" adds the CLI's usual set (implies --oauth)`)
	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "Print the login URL instead of opening a browser (implies --oauth)")

	return cmd
}

// hasTerminal reports whether prompts can be shown: stdout or stdin is a
// terminal (huh reads the controlling TTY, so a piped stdout is fine).
func hasTerminal(f *cmdutil.Factory) bool {
	if f.IOStreams.IsTTY {
		return true
	}
	file, ok := f.IOStreams.In.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func runLogin(ctx context.Context, f *cmdutil.Factory, opts loginOptions, nonInteractive bool) error {
	// The login builds its own config. Take the global overrides and clear
	// them so Printer/Config do not resolve the active profile against the
	// new server (false header warnings, leaked output_format).
	apiKey, err := f.APIKeyFromFlags()
	if err != nil {
		return err
	}
	server := f.ServerOverride
	if opts.relogin {
		server = opts.reloginServer
	}
	f.ServerOverride, f.APIKeyOverride, f.APIKeyFile = "", "", ""

	var (
		authMethod string
		username   string
		password   string
	)
	profileName := opts.name
	defProject := opts.defaultProject

	clientID, clientSecret := opts.clientID, opts.clientSecret
	switch {
	case apiKey != "":
		authMethod = "apikey"
	case opts.oauth:
		authMethod = config.AuthMethodOAuth
	}

	// Step 1: Server URL and auth method, for whatever the flags did not give
	var fields []huh.Field
	if server == "" {
		fields = append(fields, huh.NewInput().
			Title("Redmine Server URL").
			Description("e.g., https://redmine.example.com").
			Value(&server).
			Validate(func(s string) error {
				_, err := config.NormalizeServerURL(s)
				return err
			}))
	}
	if authMethod == "" {
		fields = append(fields, huh.NewSelect[string]().
			Title("Authentication Method").
			Options(
				huh.NewOption("API Key (recommended)", "apikey"),
				huh.NewOption("Username & Password", "basic"),
				huh.NewOption("OAuth (browser login)", config.AuthMethodOAuth),
			).
			Value(&authMethod))
	}
	if len(fields) > 0 {
		if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
			return err
		}
	}

	printer := f.Printer("")

	assumedHTTPS := server != "" && config.EnsureScheme(server) != server
	normalized, err := config.NormalizeServerURL(server)
	if err != nil {
		return err
	}
	if normalized != strings.TrimSpace(server) {
		printer.Success(fmt.Sprintf("Using server URL %s", normalized))
	}
	server = normalized

	// Derive default profile name from server URL
	defaultName := config.ProfileNameFromURL(server)
	if defaultName == "" {
		defaultName = "default"
	}

	// Step 2: Profile name (with default from hostname)
	if profileName == "" {
		if !nonInteractive {
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
		}
		if profileName == "" {
			profileName = defaultName
		}
	}

	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}
	var existing *config.Config
	if pc, loadErr := config.LoadProfiles(configPath, f.DebugLogger()); loadErr == nil {
		if p, ok := pc.Profiles[profileName]; ok {
			existing = &p
		}
	}

	// Pointed out before the credentials are entered.
	warnCleartextServer(f, server)

	// Step 3: Credentials
	if authMethod == config.AuthMethodOAuth {
		if clientID == "" && existing != nil && existing.AuthMethod == config.AuthMethodOAuth && config.SameServerHost(existing.Server, server) {
			clientID = existing.OAuth.ClientID
		}
		var fields []huh.Field
		if opts.clientID == "" {
			fields = append(fields, huh.NewInput().
				Title("Client ID").
				Description("Administration > Applications").
				Value(&clientID).
				Validate(requireValue("client ID")))
		}
		if clientSecret == "" {
			fields = append(fields, huh.NewInput().
				Title("Client secret").
				Description("Administration > Applications").
				EchoMode(huh.EchoModePassword).
				Value(&clientSecret).
				Validate(requireValue("client secret")))
		}
		if len(fields) > 0 {
			if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
				return err
			}
		}
	} else if apiKey == "" {
		if authMethod == "apikey" {
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("API Key").
						Description("Found at My Account > API access key").
						EchoMode(huh.EchoModePassword).
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
	}

	// --header values are merged over the profile's existing headers, used for
	// the connection test and saved, so a server that requires them is
	// reachable from the very first login. Existing headers may carry secrets
	// for the old server, so they are only reused while the host is unchanged.
	var existingHeaders map[string]string
	if existing != nil && len(existing.Headers) > 0 {
		if config.SameServerHost(existing.Server, server) {
			existingHeaders = existing.Headers
		} else {
			printer.Warning(fmt.Sprintf("Profile %q pointed at a different server; its custom headers were not carried over", profileName))
		}
	}
	headers, err := config.MergeHeaders(existingHeaders, f.Headers)
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
		Headers:    headers,
	}

	if authMethod == config.AuthMethodOAuth {
		cfg.OAuth = config.OAuthConfig{ClientID: clientID, ClientSecret: clientSecret}
		readOnly := existing != nil && existing.ReadOnly || f.ReadOnly != nil && *f.ReadOnly
		tok, err := runOAuthFlow(ctx, f, printer, cfg, opts, readOnly)
		if err != nil {
			return err
		}
		cfg.OAuth.AccessToken = tok.AccessToken
		cfg.OAuth.RefreshToken = tok.RefreshToken
		cfg.OAuth.Expiry = tok.Expiry
		cfg.OAuth.Scopes = tok.Scopes
	}

	stop := printer.Spinner("Testing connection...")

	client, err := api.NewClient(cfg, f.DebugLogger())
	if err != nil {
		stop()
		return fmt.Errorf("failed to create client: %w", err)
	}

	user, err := client.Users.Current(ctx, nil)
	stop()
	if err != nil {
		printer.Error("Connection failed: " + cmdutil.FormatErrorMessage(err))
		return loginConnectError(err, authMethod, assumedHTTPS)
	}

	printer.Success(fmt.Sprintf("Connected as %s %s (%s)", user.FirstName, user.LastName, user.Login))

	// Step 5: Default project (optional)
	var projects []models.Project
	switch {
	case defProject != "":
		if _, err := client.Projects.Get(ctx, defProject, nil); err != nil {
			return fmt.Errorf("default project %q: %s", defProject, cmdutil.FormatErrorMessage(err))
		}
	case !nonInteractive:
		stop = printer.Spinner("Fetching projects...")
		projects, _, err = client.Projects.List(ctx, nil, 100, 0)
		stop()
	}

	if err == nil && len(projects) > 0 && defProject == "" {
		options := []huh.Option[string]{
			huh.NewOption("(none)", ""),
		}
		for _, p := range projects {
			options = append(options, huh.NewOption(output.SanitizeTerminal(p.Name), p.Identifier))
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

	// A non-interactive re-login (e.g. to refresh the key) keeps the
	// profile's default project unless --default-project replaces it.
	if nonInteractive && defProject == "" && existing != nil && config.SameServerHost(existing.Server, server) {
		defProject = existing.DefaultProject
	}
	cfg.DefaultProject = defProject
	cfg.OutputFormat = "table"

	// Step 6: Credential storage choice
	storeInKeyring, err := resolveKeyringChoice(f, profileName, existing, opts.useKeyring, opts.keyringSet, nonInteractive)
	if err != nil {
		return err
	}
	if storeInKeyring {
		cfg.CredentialStore = config.CredentialStoreKeyring
	} else if nonInteractive && !opts.keyringSet && secrets.Default.Available() {
		f.Warn("credentials are stored in the plaintext config file; pass --keyring to use the system keyring instead")
	}

	// Step 7: Save profile
	if err := config.SaveProfile(profileName, cfg, configPath); err != nil {
		return fmt.Errorf("saving profile: %w", err)
	}

	if existing != nil && !storeInKeyring {
		warnSkippedKeyringCleanup(printer, existing)
	}

	// Set as active profile
	if err := config.SetActiveProfile(profileName, configPath); err != nil {
		return fmt.Errorf("setting active profile: %w", err)
	}

	printer.Action(output.ActionLoggedIn, "profile", profileName,
		fmt.Sprintf("Profile %q saved and activated (%s)", profileName, configPath))

	return nil
}

// resolveKeyringChoice decides whether to store credentials in the system
// keyring. The flag wins, then the existing profile's storage is the default;
// a keyring profile is never silently downgraded to plaintext.
func resolveKeyringChoice(f *cmdutil.Factory, profileName string, existing *config.Config, keyringFlag, keyringFlagSet, noPrompt bool) (bool, error) {
	if keyringFlagSet {
		if keyringFlag && !secrets.Default.Available() {
			return false, fmt.Errorf("system keyring is not available on this machine; re-run without --keyring or store credentials via environment variables")
		}
		return keyringFlag, nil
	}

	wasKeyring := existing != nil && existing.CredentialStore == config.CredentialStoreKeyring
	available := secrets.Default.Available()

	if wasKeyring && !available {
		return false, fmt.Errorf("profile %q stores credentials in the system keyring, which is not available right now; make the keyring usable, or re-run with --keyring=false to switch to plaintext storage", profileName)
	}
	if !f.IOStreams.IsTTY || noPrompt {
		return wasKeyring, nil
	}
	if !available {
		return false, nil
	}

	confirm := true
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Store credentials in system keyring?").
				Description("Keeps secrets out of the plaintext config file").
				Value(&confirm),
		),
	).Run(); err != nil {
		return false, err
	}
	return confirm, nil
}

// warnCleartextServer warns when the server URL would send the credentials
// in cleartext (api.IsCleartextServer). Plain http:// is supported, so this
// is the only place the CLI points it out.
func warnCleartextServer(f *cmdutil.Factory, server string) {
	if api.IsCleartextServer(server) {
		f.Warn(api.CleartextServerWarning(server))
	}
}

// loginForbiddenHint replaces the generic forbidden hint for the login probe.
// Every logged-in user may read /users/current.json, so a 403 there is never
// a missing role permission: Redmine answers it with 403 while the REST API is
// disabled, and a firewall or proxy in front of Redmine can do the same.
const loginForbiddenHint = `Redmine permissions are not the cause: every logged-in user may read their own account. Either the REST API is disabled (an administrator must enable Administration > Settings > API > Enable REST web service), or a firewall or proxy in front of Redmine blocks the request; then retry with the headers it expects, e.g. --header "User-Agent: Mozilla/5.0"`

// loginUnauthorizedHint replaces the generic 401 hint, which would point the
// user at 'redmine auth login' while they are running it.
const (
	loginUnauthorizedAPIKeyHint = `Check the API key (My account > API access key) and run the login again; a locked account is rejected the same way.`
	loginUnauthorizedBasicHint  = `Check the username and password and run the login again; a locked account is rejected the same way.`
	loginUnauthorizedOAuthHint  = `Redmine rejected the new token; check that the application and the account are active, then run the login again.`
)

// loginNotFoundHint is attached when the server answers but has no
// /users/current.json: the URL points at something other than Redmine's base.
const loginNotFoundHint = `The server answered, but not as a Redmine REST API. Check the server URL, including the sub-path the instance is installed under (e.g. https://host/redmine).`

// loginUnreachableHint is attached when the probe got no HTTP response.
const loginUnreachableHint = `Check the server URL and that the host is reachable from this machine (DNS, VPN, proxy or firewall).`

// loginHTTPSAssumedHint is appended to the unreachable hint when the login
// added https:// to a server URL typed without a scheme.
const loginHTTPSAssumedHint = ` The URL had no scheme, so https:// was assumed; if the server only serves plain HTTP, pass http:// explicitly.`

// loginConnectError wraps a failed connection probe, attaching the
// login-specific hints. authMethod selects the 401 wording; assumedHTTPS
// reports that the scheme was added by the login.
func loginConnectError(err error, authMethod string, assumedHTTPS bool) error {
	wrapped := fmt.Errorf("could not connect to Redmine server: %w", err)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		hint := loginUnreachableHint
		if assumedHTTPS {
			hint += loginHTTPSAssumedHint
		}
		return cmdutil.WithHint(wrapped, hint)
	}
	switch {
	case apiErr.IsAuthError():
		if authMethod == config.AuthMethodOAuth {
			return cmdutil.WithHint(wrapped, loginUnauthorizedOAuthHint)
		}
		if authMethod == "basic" {
			return cmdutil.WithHint(wrapped, loginUnauthorizedBasicHint)
		}
		return cmdutil.WithHint(wrapped, loginUnauthorizedAPIKeyHint)
	case apiErr.IsForbidden():
		return cmdutil.WithHint(wrapped, loginForbiddenHint)
	case apiErr.IsNotFound():
		return cmdutil.WithHint(wrapped, loginNotFoundHint)
	}
	return wrapped
}
