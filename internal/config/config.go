package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aarondpn/redmine-cli/internal/debug"
	"github.com/spf13/viper"
)

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".redmine-cli.yaml")
}

// Load reads configuration from file, environment variables, and defaults.
func Load(configPath string, log *debug.Logger) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("auth_method", "apikey")
	v.SetDefault("output_format", "table")

	// Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
		log.Printf("Config: using explicit path %s", configPath)
	} else {
		v.SetConfigName(".redmine-cli")
		v.SetConfigType("yaml")
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(home)
		}
		v.AddConfigPath(".")
	}

	// Environment variables
	v.SetEnvPrefix("REDMINE")
	v.AutomaticEnv()

	// Read config file (ignore "not found" errors)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only fail on errors other than "file not found"
			if _, err2 := os.Stat(v.ConfigFileUsed()); err2 == nil {
				return nil, fmt.Errorf("reading config: %w", err)
			}
		}
		log.Printf("Config: no config file found")
	} else {
		log.Printf("Config: loaded from %s", v.ConfigFileUsed())
	}

	// Log environment variable overrides
	for _, envVar := range []string{"REDMINE_SERVER", "REDMINE_API_KEY", "REDMINE_AUTH_METHOD"} {
		if val := os.Getenv(envVar); val != "" {
			log.Printf("Config: env override %s is set", envVar)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	log.Printf("Config: server=%s auth_method=%s", cfg.Server, cfg.AuthMethod)

	return &cfg, nil
}

// Save writes configuration to the given path.
func Save(cfg *Config, path string) error {
	v := viper.New()
	v.Set("server", cfg.Server)
	v.Set("api_key", cfg.APIKey)
	v.Set("username", cfg.Username)
	v.Set("password", cfg.Password)
	v.Set("auth_method", cfg.AuthMethod)
	v.Set("default_project", cfg.DefaultProject)
	v.Set("output_format", cfg.OutputFormat)
	v.Set("no_color", cfg.NoColor)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return v.WriteConfigAs(path)
}
