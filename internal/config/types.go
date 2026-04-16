package config

// Config holds the CLI configuration for a single profile.
type Config struct {
	Server         string `mapstructure:"server" yaml:"server,omitempty"`
	APIKey         string `mapstructure:"api_key" yaml:"api_key,omitempty"`
	Username       string `mapstructure:"username" yaml:"username,omitempty"`
	Password       string `mapstructure:"password" yaml:"password,omitempty"`
	AuthMethod     string `mapstructure:"auth_method" yaml:"auth_method,omitempty"` // "apikey" or "basic"
	DefaultProject string `mapstructure:"default_project" yaml:"default_project,omitempty"`
	OutputFormat   string `mapstructure:"output_format" yaml:"output_format,omitempty"` // "table", "json", "csv"
	NoColor        bool   `mapstructure:"no_color" yaml:"no_color,omitempty"`
}

// ProfileConfig holds the top-level configuration with multiple profiles.
type ProfileConfig struct {
	ActiveProfile string            `yaml:"active_profile,omitempty"`
	Profiles      map[string]Config `yaml:"profiles,omitempty"`
}
