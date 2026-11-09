// Package runner executes extension processes. It implements the execution
// protocol shared by the CLI and the MCP server: the environment contract
// (Env), bounded runs of declarative commands with JSON on stdin (Run), and
// passthrough runs with the caller's stdio attached (RunAttached).
//
// Credentials are never passed to an extension: it calls Redmine through
// "$REDMINE_CLI_BIN api ...", which resolves them like any other invocation
// from the forwarded profile, config file, server and headers.
package runner

import (
	"runtime"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// Environment variables of the extension protocol. The ones without a
// REDMINE_EXT_ prefix are the regular redmine-cli variables, so a nested
// "redmine api" call picks them up without any extension-specific code.
const (
	EnvCLIBin        = "REDMINE_CLI_BIN"
	EnvExtName       = "REDMINE_EXT_NAME"
	EnvExtCommand    = "REDMINE_EXT_COMMAND"
	EnvExtSurface    = "REDMINE_EXT_SURFACE"
	EnvMCP           = "REDMINE_MCP"
	EnvMCPTool       = "REDMINE_MCP_TOOL"
	EnvProfile       = config.ProfileEnv
	EnvConfig        = config.ConfigEnv
	EnvServer        = "REDMINE_SERVER"
	EnvHeaders       = config.HeadersEnv
	EnvReadOnly      = "REDMINE_READ_ONLY"
	EnvNoColor       = "REDMINE_NO_COLOR"
	EnvNoUpdateCheck = "REDMINE_NO_UPDATE_CHECK"
)

// Surface is where an extension command was invoked from.
type Surface string

const (
	SurfaceCLI Surface = "cli"
	SurfaceMCP Surface = "mcp"
)

// Env holds the inputs of the environment contract. It is a plain struct so
// the CLI (see FromFactory) and the MCP server fill it the same way; the
// zero value of every field means "not set by this invocation".
type Env struct {
	// CLIBin is the absolute path of the running redmine binary.
	CLIBin string
	// Extension is the extension name.
	Extension string
	// Command is the manifest command being run; empty for an extension
	// without a manifest.
	Command string
	// Surface is cli or mcp. Empty means cli.
	Surface Surface
	// Tool is the MCP tool name; only exported on the MCP surface.
	Tool string

	// Profile is the effective profile name.
	Profile string
	// ConfigPath is the config file selected by --config or REDMINE_CONFIG.
	ConfigPath string
	// Server is the --server override.
	Server string
	// Headers are the "Name: value" overrides of this invocation, in merge
	// order (REDMINE_HEADERS entries, then --header flags). Profile headers
	// are not included: the nested call reads them from the same profile.
	Headers []string
	// ReadOnly is set when read-only mode is active for the invocation or
	// the command declares writes: false.
	ReadOnly bool
	// ReadOnlyDisabled is set when the invocation turned read-only mode off
	// explicitly (--read-only=false). Without ReadOnly it is exported as
	// REDMINE_READ_ONLY=0, so a nested call that loads the same profile
	// does not turn read-only mode back on from the profile's read_only.
	ReadOnlyDisabled bool
	// NoColor mirrors --no-color.
	NoColor bool
}

// managedKeys are the variables Environ owns: they are always replaced or
// removed, so values inherited from an outer extension run (e.g. an
// extension calling another one) cannot leak into this one.
var managedKeys = []string{EnvCLIBin, EnvExtName, EnvExtCommand, EnvExtSurface, EnvMCP, EnvMCPTool, EnvNoUpdateCheck}

// Vars returns the contract variables for e as KEY=value entries, in a
// stable order. Variables of the "if set" kind are only included when e
// sets them.
func (e Env) Vars() []string {
	surface := e.Surface
	if surface == "" {
		surface = SurfaceCLI
	}
	vars := []string{
		EnvCLIBin + "=" + e.CLIBin,
		EnvExtName + "=" + e.Extension,
	}
	if e.Command != "" {
		vars = append(vars, EnvExtCommand+"="+e.Command)
	}
	vars = append(vars, EnvExtSurface+"="+string(surface))
	if surface == SurfaceMCP {
		vars = append(vars, EnvMCP+"=1")
		if e.Tool != "" {
			vars = append(vars, EnvMCPTool+"="+e.Tool)
		}
	}
	if e.Profile != "" {
		vars = append(vars, EnvProfile+"="+e.Profile)
	}
	if e.ConfigPath != "" {
		vars = append(vars, EnvConfig+"="+e.ConfigPath)
	}
	if e.Server != "" {
		vars = append(vars, EnvServer+"="+e.Server)
	}
	if len(e.Headers) > 0 {
		vars = append(vars, EnvHeaders+"="+config.FormatHeadersEnv(e.Headers))
	}
	switch {
	case e.ReadOnly:
		vars = append(vars, EnvReadOnly+"=1")
	case e.ReadOnlyDisabled:
		vars = append(vars, EnvReadOnly+"=0")
	}
	if e.NoColor {
		vars = append(vars, EnvNoColor+"=1")
	}
	vars = append(vars, EnvNoUpdateCheck+"=1")
	return vars
}

// Environ returns base (usually os.Environ()) with the contract applied:
// every variable from Vars replaces an inherited one of the same name, and
// managed variables this invocation does not set are removed. Everything
// else, including REDMINE_API_KEY and friends, is inherited unchanged.
//
// One inherited value is overridden although Vars does not set it: when
// read-only mode is off for this invocation but base enables it, the
// invocation turned it off explicitly (--read-only=false), so the nested
// call gets REDMINE_READ_ONLY=0 instead of the stale value.
func (e Env) Environ(base []string) []string {
	vars := e.Vars()
	if !e.ReadOnly && !e.ReadOnlyDisabled {
		if v, ok := lookup(base, EnvReadOnly); ok && config.ParseBoolEnv(v) {
			vars = append(vars, EnvReadOnly+"=0")
		}
	}
	drop := map[string]bool{}
	for _, k := range managedKeys {
		drop[envKey(k)] = true
	}
	for _, kv := range vars {
		drop[envKey(kv[:strings.IndexByte(kv, '=')])] = true
	}
	out := make([]string, 0, len(base)+len(vars))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if drop[envKey(k)] {
			continue
		}
		out = append(out, kv)
	}
	return append(out, vars...)
}

// lookup finds key in an environment list.
func lookup(env []string, key string) (string, bool) {
	want := envKey(key)
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if envKey(k) == want {
			return v, true
		}
	}
	return "", false
}

// envKey normalizes a variable name for comparison: Windows environment
// names are case-insensitive.
func envKey(k string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(k)
	}
	return k
}
