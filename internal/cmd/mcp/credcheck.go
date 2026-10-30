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
// a valid login, so both 401 and 403 point at the credentials (or at API
// access being disabled for them) rather than at a missing project role.
func credentialWarning(cfg *config.Config, err error) string {
	var ae *api.APIError
	if !errors.As(err, &ae) || (!ae.IsAuthError() && !ae.IsForbidden()) {
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
	secretVar, secret, missing := "REDMINE_API_KEY", "API key", cfg != nil && cfg.APIKey == ""
	if cfg != nil && cfg.AuthMethod == "basic" {
		secretVar, secret, missing = "REDMINE_PASSWORD", "password", cfg.Password == ""
	}
	fix := fmt.Sprintf("Run 'redmine auth login' to update the credentials, or fix %s in the MCP host config, then restart the MCP server.", secretVar)
	if ae.IsForbidden() {
		return fmt.Sprintf("Redmine at %s denied the credentials (HTTP 403): the account may be locked or REST API access disabled for it. %s", target, fix)
	}
	if missing {
		return fmt.Sprintf("Redmine at %s requires authentication (HTTP 401) but no %s is configured. %s", target, secret, fix)
	}
	return fmt.Sprintf("Redmine at %s rejected the credentials (HTTP 401): the %s is wrong, expired or revoked. %s", target, secret, fix)
}
