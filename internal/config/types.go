package config

// Config holds the CLI configuration for a single profile.
type Config struct {
	Server         string    `mapstructure:"server" yaml:"server,omitempty"`
	APIKey         string    `mapstructure:"api_key" yaml:"api_key,omitempty"`
	Username       string    `mapstructure:"username" yaml:"username,omitempty"`
	Password       string    `mapstructure:"password" yaml:"password,omitempty"`
	AuthMethod     string    `mapstructure:"auth_method" yaml:"auth_method,omitempty"` // "apikey" or "basic"
	DefaultProject string    `mapstructure:"default_project" yaml:"default_project,omitempty"`
	OutputFormat   string    `mapstructure:"output_format" yaml:"output_format,omitempty"` // "table", "json", "csv"
	NoColor        bool      `mapstructure:"no_color" yaml:"no_color,omitempty"`
	MCP            MCPConfig `mapstructure:"mcp" yaml:"mcp,omitempty"`
}

// MCPConfig holds per-profile defaults for the `redmine mcp serve` command.
// Empty slices and a nil EnableWrites mean "no preference"; the CLI flags and
// environment variables take precedence over these values.
type MCPConfig struct {
	// EnableWrites, when set, becomes the default for `--enable-writes` on
	// the MCP server. A nil pointer means "fall back to the flag default".
	EnableWrites *bool `mapstructure:"enable_writes" yaml:"enable_writes,omitempty"`
	// EnableGroups restricts the server to the listed tool groups.
	EnableGroups []string `mapstructure:"enable_groups" yaml:"enable_groups,omitempty"`
	// DisableGroups removes groups from the active set.
	DisableGroups []string `mapstructure:"disable_groups" yaml:"disable_groups,omitempty"`
	// EnableTools is an explicit allow-list of tool names.
	EnableTools []string `mapstructure:"enable_tools" yaml:"enable_tools,omitempty"`
	// DisableTools is a deny-list of tool names applied last.
	DisableTools []string `mapstructure:"disable_tools" yaml:"disable_tools,omitempty"`
	// AuthToken, when non-empty, becomes the default for `--auth-token` on
	// the MCP HTTP transport. Bearer tokens supplied here are used as the
	// shared secret clients must present in the Authorization header.
	AuthToken string `mapstructure:"auth_token" yaml:"auth_token,omitempty"`
}

// ProfileConfig holds the top-level configuration with multiple profiles.
type ProfileConfig struct {
	ActiveProfile string            `yaml:"active_profile,omitempty"`
	Profiles      map[string]Config `yaml:"profiles,omitempty"`
}
