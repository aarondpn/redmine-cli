package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
	"gopkg.in/yaml.v3"
)

// CredentialStoreKeyring is the credential_store value that stores the secret
// fields in the OS keyring instead of the plaintext config file.
const CredentialStoreKeyring = "keyring"

var errNoActiveProfile = errors.New("multiple profiles exist but no active profile set")

// Overrides carries CLI-flag credential values that outrank both the config
// file and the keyring. They are resolved before any keyring lookup so an
// explicit --server/--api-key never triggers a keyring read.
type Overrides struct {
	Server string
	APIKey string
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".redmine-cli.yaml")
}

// Load reads configuration and returns the active profile's Config.
// If profileName is non-empty, that profile is used instead of the active one.
// Environment variables (REDMINE_*) override file values.
func Load(configPath string, profileName string, log *debug.Logger) (*Config, error) {
	return load(configPath, profileName, false, Overrides{}, log)
}

// LoadAllowNoActiveProfile reads configuration while allowing the caller to
// recover with explicit CLI credentials when no active profile is selected.
func LoadAllowNoActiveProfile(configPath string, profileName string, log *debug.Logger) (*Config, error) {
	return load(configPath, profileName, true, Overrides{}, log)
}

// LoadWithOverrides reads configuration and applies CLI-flag overrides before
// the keyring is consulted, so a flag-supplied secret short-circuits the keyring
// lookup. It allows recovery without an active profile, mirroring
// LoadAllowNoActiveProfile.
func LoadWithOverrides(configPath string, profileName string, ov Overrides, log *debug.Logger) (*Config, error) {
	return load(configPath, profileName, true, ov, log)
}

func load(configPath string, profileName string, allowNoActiveProfile bool, ov Overrides, log *debug.Logger) (*Config, error) {
	if configPath == "" {
		configPath = DefaultConfigPath()
	}

	pc, err := LoadProfiles(configPath, log)
	if err != nil {
		// If no config file exists, return defaults with env overrides
		if os.IsNotExist(err) {
			log.Printf("Config: no config file found")
			cfg := &Config{
				AuthMethod:   "apikey",
				OutputFormat: "table",
			}
			applyEnvOverrides(cfg, log)
			applyOverrides(cfg, ov)
			return cfg, nil
		}
		return nil, err
	}

	// Determine which profile to use
	name := profileName
	if name == "" {
		name = pc.ActiveProfile
	}

	var cfg Config
	var selectedName string
	if name != "" {
		p, ok := pc.Profiles[name]
		if !ok {
			return nil, fmt.Errorf("profile %q not found. Run 'redmine auth list' to see available profiles", name)
		}
		cfg = p
		selectedName = name
		log.Printf("Config: loaded profile %q from %s", name, configPath)
	} else if len(pc.Profiles) == 1 {
		// Single profile, use it even without active_profile set
		for n, p := range pc.Profiles {
			cfg = p
			selectedName = n
			log.Printf("Config: loaded only profile %q from %s", n, configPath)
		}
	} else if len(pc.Profiles) == 0 {
		log.Printf("Config: no profiles configured")
	} else {
		// Apply env overrides first so explicit environment credentials can bypass
		// profile selection when requested.
		applyEnvOverrides(&cfg, log)
		if cfg.Server == "" && !allowNoActiveProfile {
			return nil, fmt.Errorf("%w. Run 'redmine auth switch' to select one", errNoActiveProfile)
		}
		if cfg.Server != "" || allowNoActiveProfile {
			log.Printf("Config: proceeding without active profile selection")
		}
	}

	// Apply env overrides (may have been applied above, idempotent)
	applyEnvOverrides(&cfg, log)

	// Apply defaults
	if cfg.AuthMethod == "" {
		cfg.AuthMethod = "apikey"
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "table"
	}

	// CLI flag overrides outrank file and env and must land before the keyring
	// lookup so an explicit --api-key/--server never triggers a keyring read.
	applyOverrides(&cfg, ov)

	// Keyring is consulted only after file/env/flag resolution, and only for
	// fields still empty. This keeps CI (which sets REDMINE_* env vars) and
	// explicit CLI flags from ever touching the keyring.
	resolveKeyringSecrets(&cfg, log)

	// When a profile opted into the keyring but the required secret could not be
	// retrieved and no higher-precedence source supplied it, fail with a clear,
	// actionable message rather than a confusing downstream 401. CLI overrides
	// have already been applied above, so a valid --api-key makes this false;
	// passing only --server against an unreachable keyring still surfaces here.
	if keyringSecretMissing(&cfg) {
		field := "REDMINE_API_KEY"
		if cfg.AuthMethod == "basic" {
			field = "REDMINE_PASSWORD"
		}
		return nil, fmt.Errorf("profile %q stores credentials in the system keyring, but the secret could not be retrieved. Re-run 'redmine auth login', set %s, or set REDMINE_NO_KEYRING=1 to disable keyring lookups", selectedName, field)
	}

	return &cfg, nil
}

// applyOverrides applies CLI-flag credential values onto cfg. Empty overrides
// leave the existing value untouched.
func applyOverrides(cfg *Config, ov Overrides) {
	if ov.Server != "" {
		cfg.Server = ov.Server
	}
	if ov.APIKey != "" {
		cfg.APIKey = ov.APIKey
	}
}

// resolveKeyringSecrets populates the auth method's secret field from the
// keyring when the profile opted in and no higher-precedence source supplied it.
// Only the field the auth method actually uses is queried, so a satisfied
// credential (or an unrelated one) never triggers a keyring access. Any keyring
// failure is logged and left for the caller's missing-credentials handling; it
// never aborts the load.
func resolveKeyringSecrets(cfg *Config, log *debug.Logger) {
	if cfg.CredentialStore != CredentialStoreKeyring || cfg.KeyringID == "" {
		return
	}

	field := secrets.FieldAPIKey
	target := &cfg.APIKey
	if cfg.AuthMethod == "basic" {
		field = secrets.FieldPassword
		target = &cfg.Password
	}
	if *target != "" {
		return
	}

	secret, ok, err := secrets.Default.Get(cfg.KeyringID, field)
	if err != nil {
		log.Printf("Config: keyring lookup for %s failed: %v", field, err)
		return
	}
	if ok {
		*target = secret
		log.Printf("Config: loaded %s from system keyring", field)
	}
}

// keyringSecretMissing reports whether a keyring profile is still missing the
// secret required by its auth method.
func keyringSecretMissing(cfg *Config) bool {
	if cfg.CredentialStore != CredentialStoreKeyring {
		return false
	}
	if cfg.AuthMethod == "basic" {
		return cfg.Password == ""
	}
	return cfg.APIKey == ""
}

// IsNoActiveProfileError reports whether err is the missing-active-profile error.
func IsNoActiveProfileError(err error) bool {
	return errors.Is(err, errNoActiveProfile)
}

// EffectiveProfileName resolves which profile name should be displayed or used
// for commands that mirror Load's profile selection behavior.
func EffectiveProfileName(pc *ProfileConfig, override string) string {
	if override != "" {
		return override
	}
	if pc == nil {
		return ""
	}
	if pc.ActiveProfile != "" {
		return pc.ActiveProfile
	}
	if len(pc.Profiles) == 1 {
		for name := range pc.Profiles {
			return name
		}
	}
	return ""
}

// LoadProfiles reads the full profile configuration from disk.
// It handles both legacy flat format and new profile format.
// If the config file does not exist, it returns an empty ProfileConfig.
func LoadProfiles(configPath string, log *debug.Logger) (*ProfileConfig, error) {
	if configPath == "" {
		configPath = DefaultConfigPath()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProfileConfig{Profiles: make(map[string]Config)}, nil
		}
		return nil, err
	}

	// Best-effort tighten of credential files written 0o644 by older versions.
	if info, statErr := os.Stat(configPath); statErr == nil && info.Mode().Perm()&0o077 != 0 {
		_ = os.Chmod(configPath, 0o600)
	}

	// Try to detect format by checking for "profiles" key
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if _, hasProfiles := raw["profiles"]; hasProfiles {
		// New profile format
		var pc ProfileConfig
		if err := yaml.Unmarshal(data, &pc); err != nil {
			return nil, fmt.Errorf("parsing config profiles: %w", err)
		}
		if pc.Profiles == nil {
			pc.Profiles = make(map[string]Config)
		}
		return &pc, nil
	}

	// Legacy flat format — convert to profile
	var legacy Config
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parsing legacy config: %w", err)
	}

	profileName := ProfileNameFromURL(legacy.Server)
	if profileName == "" {
		profileName = "default"
	}

	pc := &ProfileConfig{
		ActiveProfile: profileName,
		Profiles: map[string]Config{
			profileName: legacy,
		},
	}

	// Auto-migrate: write back in new format
	log.Printf("Config: migrating legacy format to profile %q", profileName)
	if err := SaveProfiles(pc, configPath); err != nil {
		log.Printf("Config: migration write failed: %v", err)
		// Non-fatal: still return the parsed config
	}

	return pc, nil
}

// SaveProfiles writes the full profile configuration to disk.
func SaveProfiles(pc *ProfileConfig, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// For keyring profiles, push non-empty secrets into the keyring and strip
	// them from the copy that gets marshaled so no plaintext lands on disk.
	// Empty secret fields are left untouched so re-saving a loaded keyring
	// profile does not clobber the stored value.
	toWrite := *pc
	toWrite.Profiles = make(map[string]Config, len(pc.Profiles))
	for name, cfg := range pc.Profiles {
		if cfg.CredentialStore == CredentialStoreKeyring {
			if cfg.KeyringID == "" {
				id, err := newKeyringID()
				if err != nil {
					return err
				}
				cfg.KeyringID = id
				// Record the id on the source too so a second save in the same
				// process reuses it instead of orphaning the stored secret.
				src := pc.Profiles[name]
				src.KeyringID = id
				pc.Profiles[name] = src
			}
			if err := persistKeyringSecrets(cfg); err != nil {
				return err
			}
			cfg.APIKey = ""
			cfg.Password = ""
		}
		toWrite.Profiles[name] = cfg
	}

	data, err := yaml.Marshal(&toWrite)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	// Chmod because WriteFile keeps an existing file's mode (older versions wrote 0o644).
	_ = os.Chmod(path, 0o600)
	return nil
}

// persistKeyringSecrets writes a profile's non-empty secrets to the keyring
// under its opaque KeyringID.
func persistKeyringSecrets(cfg Config) error {
	if cfg.APIKey != "" {
		if err := secrets.Default.Set(cfg.KeyringID, secrets.FieldAPIKey, cfg.APIKey); err != nil {
			return fmt.Errorf("storing API key in system keyring: %w", err)
		}
	}
	if cfg.Password != "" {
		if err := secrets.Default.Set(cfg.KeyringID, secrets.FieldPassword, cfg.Password); err != nil {
			return fmt.Errorf("storing password in system keyring: %w", err)
		}
	}
	return nil
}

// newKeyringID returns a random opaque identifier for keyring entries.
func newKeyringID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating keyring id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// removeKeyringSecrets deletes both of a keyring profile's stored secrets. A
// not-found entry is not an error (Store.Delete maps it to nil); a real backend
// failure is returned so callers can surface it rather than orphan the secret.
func removeKeyringSecrets(cfg Config) error {
	if cfg.CredentialStore != CredentialStoreKeyring || cfg.KeyringID == "" {
		return nil
	}
	if err := secrets.Default.Delete(cfg.KeyringID, secrets.FieldAPIKey); err != nil {
		return err
	}
	return secrets.Default.Delete(cfg.KeyringID, secrets.FieldPassword)
}

// Save writes a single profile's configuration (used by auth login).
func Save(cfg *Config, path string) error {
	// Load existing profiles or create new
	log := debug.New(nil)
	pc, err := LoadProfiles(path, log)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		pc = &ProfileConfig{Profiles: make(map[string]Config)}
	}

	name := ProfileNameFromURL(cfg.Server)
	if name == "" {
		name = "default"
	}

	entry, orphan := reconcileProfileEntry(pc, name, cfg)
	pc.Profiles[name] = entry
	if pc.ActiveProfile == "" {
		pc.ActiveProfile = name
	}

	if err := SaveProfiles(pc, path); err != nil {
		return err
	}
	return cleanupOrphanedKeyring(name, orphan)
}

// SaveProfile writes a named profile to the config file.
func SaveProfile(name string, cfg *Config, path string) error {
	log := debug.New(nil)
	pc, err := LoadProfiles(path, log)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		pc = &ProfileConfig{Profiles: make(map[string]Config)}
	}

	entry, orphan := reconcileProfileEntry(pc, name, cfg)
	pc.Profiles[name] = entry
	if pc.ActiveProfile == "" || len(pc.Profiles) == 1 {
		pc.ActiveProfile = name
	}

	if err := SaveProfiles(pc, path); err != nil {
		return err
	}
	return cleanupOrphanedKeyring(name, orphan)
}

// reconcileProfileEntry reconciles an incoming profile with an existing one of
// the same name, without any keyring side effects. It inherits the existing
// KeyringID when the profile stays on the keyring but arrives without one (so a
// re-login reuses the same entry instead of orphaning the old secret), and it
// returns the previous config as an orphan to clean up when a profile leaves the
// keyring for the plaintext file. Cleanup is deferred to after the config is
// durably written so an I/O failure can never destroy a still-referenced secret.
func reconcileProfileEntry(pc *ProfileConfig, name string, cfg *Config) (Config, *Config) {
	entry := *cfg
	old, had := pc.Profiles[name]
	if !had {
		return entry, nil
	}
	if entry.CredentialStore == CredentialStoreKeyring && entry.KeyringID == "" && old.KeyringID != "" {
		entry.KeyringID = old.KeyringID
	}
	if old.CredentialStore == CredentialStoreKeyring && entry.CredentialStore != CredentialStoreKeyring {
		orphan := old
		return entry, &orphan
	}
	return entry, nil
}

// cleanupOrphanedKeyring removes a profile's old keyring secret after the config
// has been committed. A backend failure is surfaced (the config change already
// stuck) rather than silently leaving the secret behind.
func cleanupOrphanedKeyring(name string, orphan *Config) error {
	if orphan == nil {
		return nil
	}
	if err := removeKeyringSecrets(*orphan); err != nil {
		return fmt.Errorf("profile %q saved, but its previous keyring credential could not be removed: %w. Unlock your keyring and remove it manually, or set REDMINE_NO_KEYRING=1", name, err)
	}
	return nil
}

// DeleteProfile removes a profile from the config file.
func DeleteProfile(name string, path string) error {
	log := debug.New(nil)
	pc, err := LoadProfiles(path, log)
	if err != nil {
		return err
	}

	deleted, ok := pc.Profiles[name]
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	delete(pc.Profiles, name)

	if pc.ActiveProfile == name {
		pc.ActiveProfile = ""
		// Only set active profile when exactly one remains
		if len(pc.Profiles) == 1 {
			for remaining := range pc.Profiles {
				pc.ActiveProfile = remaining
			}
		}
	}

	// Commit the config change before deleting the credential. If the write or
	// removal fails the secret is still intact and the profile still references
	// it, so the operation is fully retryable rather than leaving a profile that
	// points at a destroyed secret.
	if len(pc.Profiles) == 0 {
		// No profiles remain: remove the config file entirely to avoid
		// serializing an empty config that would be misinterpreted as legacy
		// format on next load.
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if err := SaveProfiles(pc, path); err != nil {
		return err
	}

	// Config is committed; the stored secret is now unreferenced. Surface a
	// backend failure so an incomplete cleanup is not silently reported as
	// success, but the profile is already gone.
	if err := removeKeyringSecrets(deleted); err != nil {
		return fmt.Errorf("profile %q removed, but its keyring credential could not be deleted: %w. Unlock your keyring and remove it manually, or set REDMINE_NO_KEYRING=1", name, err)
	}
	return nil
}

// SetActiveProfile sets the active profile in the config file.
func SetActiveProfile(name string, path string) error {
	log := debug.New(nil)
	pc, err := LoadProfiles(path, log)
	if err != nil {
		return err
	}

	if _, ok := pc.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	pc.ActiveProfile = name
	return SaveProfiles(pc, path)
}

// ProfileNameFromURL derives a profile name from a server URL.
func ProfileNameFromURL(serverURL string) string {
	if serverURL == "" {
		return ""
	}

	u, err := url.Parse(serverURL)
	if err != nil || u.Host == "" {
		// Try adding scheme
		u, err = url.Parse("https://" + serverURL)
		if err != nil || u.Host == "" {
			return strings.ReplaceAll(serverURL, "/", "-")
		}
	}

	host := u.Hostname()
	// Remove common prefixes/suffixes for cleaner names
	host = strings.TrimPrefix(host, "www.")

	return strings.ReplaceAll(host, ".", "-")
}

// applyEnvOverrides applies REDMINE_* environment variables to the config.
func applyEnvOverrides(cfg *Config, log *debug.Logger) {
	envMap := map[string]*string{
		"REDMINE_SERVER":          &cfg.Server,
		"REDMINE_API_KEY":         &cfg.APIKey,
		"REDMINE_AUTH_METHOD":     &cfg.AuthMethod,
		"REDMINE_USERNAME":        &cfg.Username,
		"REDMINE_PASSWORD":        &cfg.Password,
		"REDMINE_DEFAULT_PROJECT": &cfg.DefaultProject,
		"REDMINE_OUTPUT_FORMAT":   &cfg.OutputFormat,
	}

	for envVar, field := range envMap {
		if val := os.Getenv(envVar); val != "" {
			*field = val
			log.Printf("Config: env override %s is set", envVar)
		}
	}

	if os.Getenv("REDMINE_NO_COLOR") != "" {
		cfg.NoColor = true
	}

	mcpListEnv := map[string]*[]string{
		"REDMINE_MCP_ENABLE_GROUPS":  &cfg.MCP.EnableGroups,
		"REDMINE_MCP_DISABLE_GROUPS": &cfg.MCP.DisableGroups,
		"REDMINE_MCP_ENABLE_TOOLS":   &cfg.MCP.EnableTools,
		"REDMINE_MCP_DISABLE_TOOLS":  &cfg.MCP.DisableTools,
	}
	for envVar, field := range mcpListEnv {
		if val := os.Getenv(envVar); val != "" {
			*field = splitCSV(val)
			log.Printf("Config: env override %s is set", envVar)
		}
	}

	if val := os.Getenv("REDMINE_MCP_ENABLE_WRITES"); val != "" {
		enable := parseBoolEnv(val)
		cfg.MCP.EnableWrites = &enable
		log.Printf("Config: env override REDMINE_MCP_ENABLE_WRITES is set")
	}

	if val := os.Getenv("REDMINE_MCP_AUTH_TOKEN"); val != "" {
		cfg.MCP.AuthToken = val
		log.Printf("Config: env override REDMINE_MCP_AUTH_TOKEN is set")
	}
}

// splitCSV trims, splits on commas, and drops empty entries.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// parseBoolEnv treats common falsy values as false; everything else is true.
func parseBoolEnv(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "false", "no", "off", "":
		return false
	default:
		return true
	}
}
