package config

// Config holds the CLI configuration.
type Config struct {
	Server         string `mapstructure:"server"`
	APIKey         string `mapstructure:"api_key"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	AuthMethod     string `mapstructure:"auth_method"` // "apikey" or "basic"
	DefaultProject string `mapstructure:"default_project"`
	OutputFormat   string `mapstructure:"output_format"` // "table", "wide", "json", "csv"
	NoColor        bool   `mapstructure:"no_color"`
}
