package runner

import (
	"os"
	"path/filepath"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// Executable returns the absolute path of the running redmine binary, the
// value of REDMINE_CLI_BIN. It is a variable so tests can point it at a
// fixture.
var Executable = func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

// FromFactory fills the invocation-level fields of Env (binary, profile,
// config file, server, headers, read-only mode, color) from f once the
// global flags are applied. Extension, Command, Surface and Tool are left to
// the caller, who also turns on ReadOnly for writes: false commands.
//
// It reads the config file directly instead of calling f.Config: that would
// resolve credentials (keyring lookups, reading --api-key-file from stdin)
// that are never forwarded, and fail for extensions that work without a
// configured Redmine.
func FromFactory(f *cmdutil.Factory) (Env, error) {
	bin, err := Executable()
	if err != nil {
		return Env{}, err
	}
	env := Env{
		CLIBin:  bin,
		Server:  f.ServerOverride,
		NoColor: f.NoColorOverride,
	}
	if f.ConfigPath != "" {
		path := config.ExpandHome(f.ConfigPath)
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		env.ConfigPath = path
	}

	pc, err := config.LoadProfiles(f.ConfigPath, f.DebugLogger())
	if err != nil {
		pc = nil
	}
	env.Profile = config.EffectiveProfileName(pc, f.ProfileOverride)
	env.ReadOnly = readOnlyActive(f, pc, env.Profile)
	env.ReadOnlyDisabled = f.ReadOnly != nil && !*f.ReadOnly

	// Without --header the inherited REDMINE_HEADERS already says it all.
	// With it, the nested call needs the merged list, since setting the
	// variable replaces the inherited entries.
	if len(f.Headers) > 0 {
		envHeaders, err := config.EnvHeaders()
		if err != nil {
			return Env{}, err
		}
		env.Headers = append(envHeaders, f.Headers...)
	}
	return env, nil
}

// readOnlyActive resolves read-only mode like config.Load does (--read-only,
// then REDMINE_READ_ONLY, then the profile) without loading credentials.
func readOnlyActive(f *cmdutil.Factory, pc *config.ProfileConfig, profile string) bool {
	if f.ReadOnly != nil {
		return *f.ReadOnly
	}
	if v := os.Getenv(EnvReadOnly); v != "" {
		return config.ParseBoolEnv(v)
	}
	if pc != nil {
		if p, ok := pc.Profiles[profile]; ok {
			return p.ReadOnly
		}
	}
	return false
}
