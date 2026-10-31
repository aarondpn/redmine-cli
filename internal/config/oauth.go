package config

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// LoadOAuthTokens returns the stored OAuth block of profile, with the
// keyring secrets resolved for a keyring profile.
func LoadOAuthTokens(path, profile string) (OAuthConfig, error) {
	log := debug.New(nil)
	pc, err := LoadProfiles(path, log)
	if err != nil {
		return OAuthConfig{}, err
	}
	cfg, ok := pc.Profiles[profile]
	if !ok {
		return OAuthConfig{}, fmt.Errorf("profile %q not found", profile)
	}
	resolveKeyringSecrets(&cfg, log)
	return cfg.OAuth, nil
}

// SaveOAuthTokens replaces the tokens, expiry and scopes of profile's OAuth
// block, keeping its client. The caller holds LockConfig. It fails when the
// profile is gone or now has another auth method or client ID (re-login or
// logout in between), so a refresh never resurrects or overwrites it.
func SaveOAuthTokens(path, profile string, tok OAuthConfig) error {
	pc, err := LoadProfiles(path, debug.New(nil))
	if err != nil {
		return err
	}
	cfg, ok := pc.Profiles[profile]
	if !ok {
		return fmt.Errorf("profile %q not found", profile)
	}
	if cfg.AuthMethod != AuthMethodOAuth || cfg.OAuth.ClientID != tok.ClientID {
		return fmt.Errorf("profile %q changed while refreshing the OAuth token; run the command again", profile)
	}
	cfg.OAuth.AccessToken = tok.AccessToken
	cfg.OAuth.RefreshToken = tok.RefreshToken
	cfg.OAuth.Expiry = tok.Expiry
	cfg.OAuth.Scopes = tok.Scopes
	pc.Profiles[profile] = cfg
	return SaveProfiles(pc, path)
}
