package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/oauth"
)

// credentialCheckTimeout bounds the startup probe so a slow or unreachable
// server cannot leave a goroutine behind for long.
const credentialCheckTimeout = 10 * time.Second

// startCredentialCheck probes the configured credentials in the background
// and warns on stderr when Redmine rejects them. It never delays the MCP
// handshake, and failures other than a clear credential rejection (network
// errors, timeouts, 5xx) stay silent: the tools report those when used. The
// returned channel closes once the probe is done.
func startCredentialCheck(ctx context.Context, f *cmdutil.Factory, cfg *config.Config, client *api.Client) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(ctx, credentialCheckTimeout)
		defer cancel()
		if _, err := client.Users.Current(ctx, nil); err != nil {
			if msg := credentialWarning(cfg, err); msg != "" {
				f.Warn(msg)
			}
		}
	}()
	return done
}

// credentialWarning returns the warning for err, or "" when err is not a
// clear credential rejection. /users/current.json needs no permission beyond
// a valid login, so a 401 points at the credentials, and a 403 at the REST
// API being disabled (or a proxy blocking the request): Redmine treats a
// locked account as anonymous and answers 401, never 403.
func credentialWarning(cfg *config.Config, err error) string {
	loginExpired := errors.Is(err, oauth.ErrLoginRequired)
	var ae *api.APIError
	if !loginExpired && (!errors.As(err, &ae) || (!ae.IsAuthError() && !ae.IsForbidden())) {
		return ""
	}
	target := "the configured Redmine server"
	if cfg != nil {
		if cfg.Server != "" {
			target = debug.ScrubURL(cfg.Server)
		}
		if profile, _, ok := cfg.ProfileServer(); ok {
			target += fmt.Sprintf(" (profile %q)", profile)
		}
	}
	if loginExpired {
		return fmt.Sprintf("Redmine at %s: the OAuth login expired or was revoked. Run '%s'; no restart needed.", target, api.ReloginCommand(err))
	}
	secretVar, secret, missing := "REDMINE_API_KEY", "API key", cfg != nil && cfg.APIKey == ""
	if cfg != nil && cfg.AuthMethod == "basic" {
		secretVar, secret, missing = "REDMINE_PASSWORD", "password", cfg.Password == ""
	}
	oauthLogin := cfg != nil && cfg.AuthMethod == config.AuthMethodOAuth
	if oauthLogin {
		secretVar, secret, missing = "", "OAuth login", cfg.OAuth.RefreshToken == ""
	}
	fix := fmt.Sprintf("Run 'redmine auth login' to update the credentials, or fix %s in the MCP host config, then restart the MCP server.", secretVar)
	if oauthLogin {
		fix = "Run 'redmine auth login --oauth' to log in again; no restart needed."
	}
	if ae.IsForbidden() {
		return fmt.Sprintf("Redmine at %s refused the credentials check (HTTP 403), which it answers for every request while the REST API is disabled: an administrator must enable Administration > Settings > API > Enable REST web service (or a proxy in front of Redmine blocks the request). Run 'redmine auth status' to check.", target)
	}
	if missing {
		return fmt.Sprintf("Redmine at %s requires authentication (HTTP 401) but no %s is configured. %s", target, secret, fix)
	}
	return fmt.Sprintf("Redmine at %s rejected the credentials (HTTP 401): the %s is wrong, expired or revoked, or the account is locked. %s", target, secret, fix)
}
