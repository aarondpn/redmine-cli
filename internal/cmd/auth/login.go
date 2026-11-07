package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	caCert         string
	caCertSet      bool
}

// loginNeedsTerminalError is returned when values are missing and there is no
// terminal to prompt on.
const loginNeedsTerminalError = `redmine auth login needs a terminal to prompt for missing values; pass them as flags instead, e.g.
  redmine auth login --server https://redmine.example.com --api-key-file -
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

For a server whose certificate is signed by a private or corporate CA, pass
--ca-cert with a PEM file; it is used for the connection test and saved to the
profile as an absolute path. Logging in again to the same server without
--ca-cert keeps the profile's ca_cert; --ca-cert="" removes it.`,
		Example: `  redmine auth login
  redmine auth login --name work
  redmine auth login --header "User-Agent: Mozilla/5.0"
  redmine auth login --ca-cert ~/certs/company-ca.pem
  echo "$KEY" | redmine auth login --server https://redmine.example.com --api-key-file -
  redmine auth login --server redmine.example.com --api-key-file ~/.redmine-key --default-project web -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nonInteractive := f.ServerOverride != "" && (f.APIKeyOverride != "" || f.APIKeyFile != "")
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
			opts.caCertSet = cmd.Flags().Changed("ca-cert")
			return runLogin(cmdutil.Context(cmd), f, opts, nonInteractive)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Profile name (default: derived from server hostname)")
	cmd.Flags().BoolVar(&opts.useKeyring, "keyring", false, "Store credentials in the system keyring instead of the plaintext config file")
	cmd.Flags().StringVar(&opts.defaultProject, "default-project", "", "Default project identifier to save in the profile")
	cmd.Flags().StringVar(&opts.caCert, "ca-cert", "", `PEM file with CA certificates to trust for this server, in addition to the system roots (--ca-cert="" removes the profile's)`)

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
	f.ServerOverride, f.APIKeyOverride, f.APIKeyFile = "", "", ""

	var (
		authMethod string
		username   string
		password   string
	)
	profileName := opts.name
	defProject := opts.defaultProject

	// --ca-cert is saved as an absolute path so later commands find it from
	// any directory. An unusable file fails here, before any prompt.
	caCertFlag := ""
	if opts.caCert != "" {
		if caCertFlag, err = filepath.Abs(config.ExpandHome(opts.caCert)); err != nil {
			return fmt.Errorf("--ca-cert: %w", err)
		}
		if _, err := api.LoadCACertPool(caCertFlag); err != nil {
			return fmt.Errorf("--ca-cert: %w", err)
		}
	}

	if apiKey != "" {
		authMethod = "apikey"
	}

	// The printer applies the config's no_color, which the login UI's
	// styles must see before they are first rendered.
	printer := f.Printer("")
	ui := newLoginUI(f.IOStreams.ErrOut, !nonInteractive)
	ui.start()

	// Step 1: Server URL, unless given as a flag. One question per form, so
	// each answer is confirmed by its step line before the next prompt.
	if server == "" {
		err := ui.run(huh.NewInput().
			Title("Server URL").
			Description("The address you open Redmine at, e.g. https://redmine.example.com").
			Prompt(loginPrompt).
			Value(&server).
			Validate(func(s string) error {
				_, err := config.NormalizeServerURL(s)
				return err
			}))
		if err != nil {
			return err
		}
	}

	assumedHTTPS := server != "" && config.EnsureScheme(server) != server
	normalized, err := config.NormalizeServerURL(server)
	if err != nil {
		return err
	}
	if normalized != strings.TrimSpace(server) && nonInteractive {
		printer.Success(fmt.Sprintf("Using server URL %s", normalized))
	}
	server = normalized
	ui.step("Server", server)

	// Derive default profile name from server URL
	defaultName := config.ProfileNameFromURL(server)
	if defaultName == "" {
		defaultName = "default"
	}

	// Step 2: Profile name (with default from hostname)
	if profileName == "" {
		if !nonInteractive {
			err = ui.run(
				huh.NewInput().
					Title("Profile name").
					Description(fmt.Sprintf("A short name for this connection; press enter to use %q", defaultName)).
					Prompt(loginPrompt).
					Placeholder(defaultName).
					Value(&profileName),
			)
			if err != nil {
				return err
			}
		}
		if profileName == "" {
			profileName = defaultName
		}
	}
	ui.step("Profile", profileName)

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

	// --ca-cert sets the profile's ca_cert (--ca-cert="" removes it). Without
	// the flag the existing one is kept while the host is unchanged, since a
	// different server likely needs a different CA. REDMINE_CA_CERT overrides
	// it for the connection test, as for every other command, but is not
	// saved; --ca-cert wins over it.
	caCert := caCertFlag
	if !opts.caCertSet && existing != nil && existing.CACert != "" {
		if config.SameServerHost(existing.Server, server) {
			caCert = existing.CACert
		} else {
			printer.Warning(fmt.Sprintf("Profile %q pointed at a different server; its ca_cert was not carried over (pass --ca-cert to set one)", profileName))
		}
	}
	testCACert, testCACertLabel := caCert, caCert
	if env := os.Getenv(config.CACertEnv); env != "" && !opts.caCertSet {
		testCACert, testCACertLabel = env, env+" ("+config.CACertEnv+")"
	}
	// --ca-cert was checked above; a stale inherited value fails here too,
	// before the credential prompts.
	if testCACert != "" && !opts.caCertSet {
		if _, err := api.LoadCACertPool(testCACert); err != nil {
			if testCACert != caCert {
				return cmdutil.WithHint(fmt.Errorf("%s: %w", config.CACertEnv, err),
					`Fix or unset `+config.CACertEnv+`, or pass --ca-cert=<file>.`)
			}
			return cmdutil.WithHint(fmt.Errorf("ca_cert of profile %q: %w", profileName, err),
				`Pass --ca-cert=<file> to replace it, or --ca-cert="" to remove it.`)
		}
	}

	// Step 3: Auth method, unless implied by an API key flag
	if authMethod == "" {
		err := ui.run(huh.NewSelect[string]().
			Title("Authentication method").
			Options(
				huh.NewOption("API key (recommended)", "apikey"),
				huh.NewOption("Username & password", "basic"),
			).
			Value(&authMethod))
		if err != nil {
			return err
		}
	}
	ui.step("Authentication", authMethodLabel(authMethod))

	// Pointed out before the credentials are entered, below the header.
	ui.flush()
	warnCleartextServer(f, server)

	// Step 4: Credentials
	if apiKey == "" {
		if authMethod == "apikey" {
			err = ui.run(
				huh.NewInput().
					Title("API key").
					Description(fmt.Sprintf("Open %s/my/account and click Show under API access key", server)).
					Prompt(loginPrompt).
					EchoMode(huh.EchoModePassword).
					Value(&apiKey).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("API key is required")
						}
						return nil
					}),
			)
		} else {
			err = ui.run(
				huh.NewInput().
					Title("Username").
					Prompt(loginPrompt).
					Value(&username).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("username is required")
						}
						return nil
					}),
				huh.NewInput().
					Title("Password").
					Prompt(loginPrompt).
					EchoMode(huh.EchoModePassword).
					Value(&password),
			)
		}
		if err != nil {
			return err
		}
		// A pasted key or name often carries a stray space or newline.
		apiKey, username = strings.TrimSpace(apiKey), strings.TrimSpace(username)
	}
	if authMethod == "basic" {
		ui.step("Username", username)
		ui.step("Password", maskSecret(password))
	} else {
		ui.step("API key", maskSecret(apiKey))
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

	// Step 5: Test connection
	cfg := &config.Config{
		Server:     server,
		AuthMethod: authMethod,
		APIKey:     apiKey,
		Username:   username,
		Password:   password,
		Headers:    headers,
	}
	cfg.CACert = caCert
	testCfg := cfg
	if testCACert != caCert {
		withEnv := *cfg
		withEnv.CACert = testCACert
		testCfg = &withEnv
	}
	if testCACertLabel != "" {
		ui.step("CA certificate", testCACertLabel)
	}

	stop := printer.Spinner("Testing connection...")

	client, err := api.NewClient(testCfg, f.DebugLogger())
	if err != nil {
		stop()
		return fmt.Errorf("failed to create client: %w", err)
	}

	user, err := client.Users.Current(ctx, nil)
	stop()
	if err != nil {
		if nonInteractive {
			printer.Error("Connection failed: " + cmdutil.FormatErrorMessage(err))
		} else {
			ui.fail("Connection", "failed")
			fmt.Fprintln(f.IOStreams.ErrOut)
		}
		return loginConnectError(err, authMethod, assumedHTTPS, testCACert)
	}

	connectedAs := fmt.Sprintf("%s %s (%s)", user.FirstName, user.LastName, user.Login)
	if nonInteractive {
		printer.Success("Connected as " + connectedAs)
	}
	ui.step("Connected as", connectedAs)

	// Step 6: Default project (optional)
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
			huh.NewOption("No default project", ""),
		}
		for _, p := range projects {
			options = append(options, huh.NewOption(output.SanitizeTerminal(p.Name), p.Identifier))
		}

		// Only a cancel stops the login; any other failure leaves no default.
		err := ui.run(
			huh.NewSelect[string]().
				Title("Default project").
				Description("Used when --project is not given; type / to filter").
				Options(options...).
				Value(&defProject).
				Height(8),
		)
		if errors.Is(err, huh.ErrUserAborted) {
			return err
		}
	}

	// A non-interactive re-login (e.g. to refresh the key) keeps the
	// profile's default project unless --default-project replaces it.
	if nonInteractive && defProject == "" && existing != nil && config.SameServerHost(existing.Server, server) {
		defProject = existing.DefaultProject
	}
	cfg.DefaultProject = defProject
	cfg.OutputFormat = "table"
	if defProject != "" {
		ui.step("Default project", projectLabel(projects, defProject))
	} else {
		ui.stepMuted("Default project", "none")
	}

	// Step 7: Credential storage choice
	storeInKeyring, err := resolveKeyringChoice(f, profileName, existing, opts.useKeyring, opts.keyringSet, nonInteractive)
	if err != nil {
		return err
	}
	if storeInKeyring {
		cfg.CredentialStore = config.CredentialStoreKeyring
	} else if nonInteractive && !opts.keyringSet && secrets.Default.Available() {
		f.Warn("credentials are stored in the plaintext config file; pass --keyring to use the system keyring instead")
	}
	if storeInKeyring {
		ui.step("Credentials", "System keyring")
	} else {
		ui.step("Credentials", "Config file (plaintext)")
	}

	// Step 8: Save profile
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

	if !nonInteractive {
		ui.done(profileName, configPath)
		return nil
	}
	printer.Action(output.ActionLoggedIn, "profile", profileName,
		fmt.Sprintf("Profile %q saved and activated (%s)", profileName, configPath))

	return nil
}

// projectLabel returns "Name (identifier)" for a project picked from the
// list, or the identifier alone when the project was not listed.
func projectLabel(projects []models.Project, identifier string) string {
	for _, p := range projects {
		if p.Identifier == identifier {
			return fmt.Sprintf("%s (%s)", p.Name, p.Identifier)
		}
	}
	return identifier
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

	useKeyring := true
	if err := runLoginForm(true,
		huh.NewSelect[bool]().
			Title("Where should the credentials be stored?").
			// Value before Options: Options preselects the option matching
			// the bound value, and the zero value false would scroll the list.
			Value(&useKeyring).
			Options(
				huh.NewOption("System keyring (recommended)", true),
				huh.NewOption("Config file (plaintext)", false),
			),
	); err != nil {
		return false, err
	}
	return useKeyring, nil
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
)

// loginNotFoundHint is attached when the server answers but has no
// /users/current.json: the URL points at something other than Redmine's base.
const loginNotFoundHint = `The server answered, but not as a Redmine REST API. Check the server URL, including the sub-path the instance is installed under (e.g. https://host/redmine).`

// loginUnreachableHint is attached when the probe got no HTTP response.
const loginUnreachableHint = `Check the server URL and that the host is reachable from this machine (DNS, VPN, proxy or firewall).`

// loginUntrustedCertHint is attached when the server's certificate could not
// be verified for a reason other than a host name mismatch or expiry.
const loginUntrustedCertHint = `The server's certificate is not trusted by this machine. If it is signed by a private or corporate CA, pass that CA's certificate as a PEM file with --ca-cert, e.g. redmine auth login --ca-cert ~/certs/company-ca.pem`

// loginWrongCAHint replaces loginUntrustedCertHint when a CA certificate was
// already used; %s is its path.
const loginWrongCAHint = `The server's certificate is not signed by a CA in %s. Check that the file holds the CA that issued it, or that CA's root.`

// loginHTTPSAssumedHint is appended to the unreachable hint when the login
// added https:// to a server URL typed without a scheme.
const loginHTTPSAssumedHint = ` The URL had no scheme, so https:// was assumed; if the server only serves plain HTTP, pass http:// explicitly.`

// isUntrustedCertError reports whether err is a failed verification of the
// server's certificate that a CA certificate could fix. On macOS the platform
// verifier does not always return a typed x509 error, so any verification
// failure counts except a host name mismatch or an expired certificate.
func isUntrustedCertError(err error) bool {
	var unknownCA x509.UnknownAuthorityError
	if errors.As(err, &unknownCA) {
		return true
	}
	var verifyErr *tls.CertificateVerificationError
	if !errors.As(err, &verifyErr) {
		return false
	}
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &hostErr) || (errors.As(err, &invalid) && invalid.Reason == x509.Expired) {
		return false
	}
	return true
}

// loginConnectError wraps a failed connection probe, attaching the
// login-specific hints. authMethod selects the 401 wording; assumedHTTPS
// reports that the scheme was added by the login; caCert is the CA file used
// for the probe, if any.
func loginConnectError(err error, authMethod string, assumedHTTPS bool, caCert string) error {
	wrapped := fmt.Errorf("could not connect to Redmine server: %w", err)
	if isUntrustedCertError(err) {
		if caCert != "" {
			return cmdutil.WithHint(wrapped, fmt.Sprintf(loginWrongCAHint, caCert))
		}
		return cmdutil.WithHint(wrapped, loginUntrustedCertHint)
	}
	// A host name mismatch or an expired certificate: the server was reached,
	// so the DNS/VPN hint below would mislead.
	var verifyErr *tls.CertificateVerificationError
	if errors.As(err, &verifyErr) {
		return wrapped
	}
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
