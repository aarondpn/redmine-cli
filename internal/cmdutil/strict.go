package cmdutil

import (
	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// StrictMode reports whether strict mode is enabled for the selected profile
// (--profile, else the active or only profile). Unlike Config it needs no
// credentials, so it still answers when Config fails, for instance because
// strict mode refused --api-key or a keyring secret is missing.
func (f *Factory) StrictMode() bool {
	return f.ProfileStrictMode("")
}

// ProfileStrictMode reports whether strict mode is enabled for the named
// profile; an empty name selects the profile like StrictMode does.
// REDMINE_STRICT can enable strict mode but never disable it. A config file
// that exists but cannot be read or parsed counts as strict, so a broken file
// never downgrades a security check to a warning.
func (f *Factory) ProfileStrictMode(name string) bool {
	if strict, _ := config.StrictEnv(); strict {
		return true
	}
	pc, err := config.LoadProfiles(f.ConfigPath, f.DebugLogger())
	if err != nil {
		return true
	}
	if name == "" {
		name = config.EffectiveProfileName(pc, f.ProfileOverride)
	}
	p, ok := pc.Profiles[name]
	return ok && p.Strict
}
