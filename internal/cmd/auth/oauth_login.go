package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/browser"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// openBrowser opens the login URL. A variable so tests can replace it.
var openBrowser = browser.Open

const (
	oauthTimeoutHint = "Check that the REST API is enabled, the application's redirect URI is http://127.0.0.1/callback and its scopes cover the request (Redmine shows scope errors only in the browser)."
	oauthScopeHint   = "Tick the scopes on the application (Administration > Applications), or pass --scopes."
	oauthClientHint  = "Check the client ID and secret (Administration > Applications)."
)

// requireValue returns a huh validator that rejects an empty answer.
func requireValue(what string) func(string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf("%s is required", what)
		}
		return nil
	}
}

// applyRelogin turns `auth login --oauth` without --server and --client-id
// into a re-login of an existing OAuth profile: it takes the profile's name,
// server, client ID and client secret. It reports whether that applied.
func applyRelogin(f *cmdutil.Factory, opts *loginOptions) bool {
	if f.ServerOverride != "" || opts.clientID != "" {
		return false
	}
	configPath := config.DefaultConfigPath()
	if f.ConfigPath != "" {
		configPath = f.ConfigPath
	}
	name := opts.name
	if name == "" {
		name = f.ProfileOverride
	}
	if name == "" {
		pc, err := config.LoadProfiles(configPath, f.DebugLogger())
		if err != nil {
			return false
		}
		name = config.EffectiveProfileName(pc, "")
	}
	if name == "" {
		return false
	}
	cfg, err := config.Load(configPath, name, f.DebugLogger())
	if err != nil || cfg.AuthMethod != config.AuthMethodOAuth || cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
		return false
	}
	opts.name = name
	opts.clientID = cfg.OAuth.ClientID
	opts.clientSecret = cfg.OAuth.ClientSecret
	opts.relogin = true
	opts.reloginServer = cfg.Server
	return true
}

// runOAuthFlow performs the browser login for cfg's server and OAuth client
// and returns the issued token.
func runOAuthFlow(ctx context.Context, f *cmdutil.Factory, printer output.Printer, cfg *config.Config, opts loginOptions, readOnly bool) (oauth.Token, error) {
	scopes := oauth.DefaultScopes(readOnly)
	if opts.scopes != "" {
		var err error
		if scopes, err = oauth.ParseScopes(opts.scopes, readOnly); err != nil {
			return oauth.Token{}, err
		}
	}

	oc, err := api.NewOAuthClient(cfg, f.DebugLogger())
	if err != nil {
		return oauth.Token{}, err
	}
	stop := printer.Spinner("Checking OAuth client...")
	err = oc.Preflight(ctx)
	stop()
	if err != nil {
		return oauth.Token{}, oauthLoginError(err)
	}

	flow, err := oc.Start(scopes)
	if err != nil {
		return oauth.Token{}, err
	}
	defer func() { _ = flow.Close() }()

	if opts.noBrowser {
		fmt.Fprintf(f.IOStreams.ErrOut, "Open this URL in a browser to log in:\n  %s\n", flow.URL())
	} else {
		fmt.Fprintf(f.IOStreams.ErrOut, "Opening the browser to log in. If it does not open, visit:\n  %s\n", flow.URL())
		_ = openBrowser(flow.URL()) // the URL is printed either way
	}

	ctx, cancel := context.WithTimeout(ctx, oauth.LoginTimeout)
	defer cancel()
	tok, err := waitForLogin(ctx, f, flow)
	if err != nil {
		return oauth.Token{}, oauthLoginError(err)
	}
	return tok, nil
}

// waitForLogin waits for the browser redirect. In a terminal it also asks for
// the redirected URL, for a browser that cannot reach this machine (SSH).
func waitForLogin(ctx context.Context, f *cmdutil.Factory, flow *oauth.Flow) (oauth.Token, error) {
	if !hasTerminal(f) {
		return flow.Wait(ctx)
	}
	type result struct {
		tok oauth.Token
		err error
	}
	promptCtx, stopPrompt := context.WithCancel(ctx)
	defer stopPrompt()
	done := make(chan result, 1)
	go func() {
		tok, err := flow.Wait(ctx)
		done <- result{tok, err}
		stopPrompt()
	}()

	var pasted string
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Paste the URL the browser ended on").
			Description("Only needed if the browser is on another machine").
			Value(&pasted).
			Validate(flow.Deliver),
	)).RunWithContext(promptCtx)
	if errors.Is(err, huh.ErrUserAborted) {
		return oauth.Token{}, err
	}
	res := <-done
	return res.tok, res.err
}

// oauthLoginError adds the hints for the common setup mistakes.
func oauthLoginError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return cmdutil.WithHint(errors.New("timed out waiting for the browser login"), oauthTimeoutHint)
	case errors.Is(err, oauth.ErrInvalidScope):
		return cmdutil.WithHint(err, oauthScopeHint)
	case errors.Is(err, oauth.ErrInvalidClient):
		return cmdutil.WithHint(err, oauthClientHint)
	}
	return err
}

// revokeOAuthLogin revokes the stored OAuth login of a profile at Redmine.
func revokeOAuthLogin(ctx context.Context, f *cmdutil.Factory, configPath, name string) error {
	cfg, err := config.Load(configPath, name, f.DebugLogger())
	if err != nil {
		return err
	}
	if cfg.OAuth.RefreshToken == "" {
		return nil
	}
	oc, err := api.NewOAuthClient(cfg, f.DebugLogger())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return oc.Revoke(ctx, cfg.OAuth.RefreshToken)
}
