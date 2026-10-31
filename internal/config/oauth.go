package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// ErrProfileChanged is returned by SaveOAuthTokens when the profile is gone
// or logged in again in the meantime.
var ErrProfileChanged = errors.New("profile changed")

type profileChangedError struct{ msg string }

func (e profileChangedError) Error() string        { return e.msg }
func (e profileChangedError) Is(target error) bool { return target == ErrProfileChanged }

// SameServer reports whether a and b are the same Redmine base URL, ignoring
// a missing scheme and trailing slashes.
func SameServer(a, b string) bool {
	norm := func(s string) string { return strings.TrimRight(EnsureScheme(s), "/") }
	return norm(a) == norm(b)
}

// LoadStoredProfile returns profile as stored, without environment or flag
// overrides, with the keyring secrets resolved for a keyring profile.
func LoadStoredProfile(path, profile string) (Config, error) {
	log := debug.New(nil)
	pc, err := LoadProfiles(path, log)
	if err != nil {
		return Config{}, err
	}
	cfg, ok := pc.Profiles[profile]
	if !ok {
		return Config{}, fmt.Errorf("profile %q not found", profile)
	}
	resolveKeyringSecrets(&cfg, log)
	return cfg, nil
}

// SaveOAuthTokens replaces the tokens, expiry and scopes of profile's OAuth
// block, keeping its client. The caller holds LockConfig. It fails when the
// profile is gone or now has another auth method, client ID or server
// (re-login or logout in between), so a refresh never resurrects or
// overwrites it. An empty server skips the server check.
func SaveOAuthTokens(path, profile, server string, tok OAuthConfig) error {
	pc, err := LoadProfiles(path, debug.New(nil))
	if err != nil {
		return err
	}
	cfg, ok := pc.Profiles[profile]
	if !ok {
		return profileChangedError{fmt.Sprintf("profile %q not found", profile)}
	}
	if cfg.AuthMethod != AuthMethodOAuth || cfg.OAuth.ClientID != tok.ClientID || server != "" && !SameServer(cfg.Server, server) {
		return profileChangedError{fmt.Sprintf("profile %q changed while refreshing the OAuth token; run the command again", profile)}
	}
	cfg.OAuth.AccessToken = tok.AccessToken
	cfg.OAuth.RefreshToken = tok.RefreshToken
	cfg.OAuth.Expiry = tok.Expiry
	cfg.OAuth.Scopes = tok.Scopes
	pc.Profiles[profile] = cfg
	return SaveProfiles(pc, path)
}
