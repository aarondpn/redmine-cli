package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aarondpn/redmine-cli/internal/debug"
	"gopkg.in/yaml.v3"
)

var errNoActiveProfile = errors.New("multiple profiles exist but no active profile set")

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".redmine-cli.yaml")
}

// Load reads configuration and returns the active profile's Config.
// If profileName is non-empty, that profile is used instead of the active one.
// Environment variables (REDMINE_*) override file values.
func Load(configPath string, profileName string, log *debug.Logger) (*Config, error) {
	return load(configPath, profileName, false, log)
}

// LoadAllowNoActiveProfile reads configuration while allowing the caller to
// recover with explicit CLI credentials when no active profile is selected.
func LoadAllowNoActiveProfile(configPath string, profileName string, log *debug.Logger) (*Config, error) {
	return load(configPath, profileName, true, log)
}

func load(configPath string, profileName string, allowNoActiveProfile bool, log *debug.Logger) (*Config, error) {
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
	if name != "" {
		p, ok := pc.Profiles[name]
		if !ok {
			return nil, fmt.Errorf("profile %q not found. Run 'redmine auth list' to see available profiles", name)
		}
		cfg = p
		log.Printf("Config: loaded profile %q from %s", name, configPath)
	} else if len(pc.Profiles) == 1 {
		// Single profile, use it even without active_profile set
		for n, p := range pc.Profiles {
			cfg = p
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

	return &cfg, nil
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

	data, err := yaml.Marshal(pc)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
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

	pc.Profiles[name] = *cfg
	if pc.ActiveProfile == "" {
		pc.ActiveProfile = name
	}

	return SaveProfiles(pc, path)
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

	pc.Profiles[name] = *cfg
	if pc.ActiveProfile == "" || len(pc.Profiles) == 1 {
		pc.ActiveProfile = name
	}

	return SaveProfiles(pc, path)
}

// DeleteProfile removes a profile from the config file.
func DeleteProfile(name string, path string) error {
	log := debug.New(nil)
	pc, err := LoadProfiles(path, log)
	if err != nil {
		return err
	}

	if _, ok := pc.Profiles[name]; !ok {
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

	// If no profiles remain, remove the config file entirely
	// to avoid serializing an empty config that would be
	// misinterpreted as legacy format on next load.
	if len(pc.Profiles) == 0 {
		return os.Remove(path)
	}

	return SaveProfiles(pc, path)
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
}
