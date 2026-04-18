package cmdutil

import (
	"io"
	"os"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"golang.org/x/term"
)

// IOStreams holds the standard I/O streams.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
	IsTTY  bool
}

// Factory provides lazy access to configuration, API client, and output printer.
type Factory struct {
	ConfigPath string
	Verbose    bool

	// Runtime overrides from CLI flags (highest precedence).
	ProfileOverride string
	ServerOverride  string
	APIKeyOverride  string
	NoColorOverride bool

	// OutputFormat is set by the root persistent --output/-o flag and used
	// as the default format when commands call Printer("").
	OutputFormat string

	config    *config.Config
	client    *api.Client
	debugLog  *debug.Logger
	IOStreams *IOStreams
}

// NewFactory creates a new Factory with default I/O streams.
func NewFactory() *Factory {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	return &Factory{
		IOStreams: &IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
			IsTTY:  isTTY,
		},
	}
}

// DebugLogger returns the debug logger, creating it on first call.
func (f *Factory) DebugLogger() *debug.Logger {
	if f.debugLog != nil {
		return f.debugLog
	}
	if f.Verbose {
		f.debugLog = debug.New(f.IOStreams.ErrOut)
	} else {
		f.debugLog = debug.New(nil)
	}
	return f.debugLog
}

// Config returns the loaded configuration (cached after first call).
// CLI flag overrides (ServerOverride, APIKeyOverride, NoColorOverride) are
// applied after loading from file and environment, giving them the highest
// precedence.
func (f *Factory) Config() (*config.Config, error) {
	if f.config != nil {
		return f.config, nil
	}
	loadFn := config.Load
	if f.ServerOverride != "" || f.APIKeyOverride != "" {
		loadFn = config.LoadAllowNoActiveProfile
	}

	cfg, err := loadFn(f.ConfigPath, f.ProfileOverride, f.DebugLogger())
	if err != nil {
		return nil, err
	}

	// Apply CLI flag overrides (highest precedence).
	if f.ServerOverride != "" {
		cfg.Server = f.ServerOverride
	}
	if f.APIKeyOverride != "" {
		cfg.APIKey = f.APIKeyOverride
	}
	if f.NoColorOverride {
		cfg.NoColor = true
	}

	f.config = cfg
	return cfg, nil
}

// ApiClient returns an API client (cached after first call).
func (f *Factory) ApiClient() (*api.Client, error) {
	if f.client != nil {
		return f.client, nil
	}
	cfg, err := f.Config()
	if err != nil {
		return nil, err
	}
	client, err := api.NewClient(cfg, f.DebugLogger())
	if err != nil {
		return nil, err
	}
	f.client = client
	return client, nil
}

// Printer creates and returns a new output printer for the given format.
func (f *Factory) Printer(format string) output.Printer {
	noColor := os.Getenv("NO_COLOR") != ""
	if format == "" {
		format = f.OutputFormat
	} else if f.OutputFormat == "" {
		// A leaf command registered its own --output via AddOutputFlag and
		// captured the user's choice into a local variable. Record it on the
		// factory so the top-level error path (and any later Printer call)
		// can see the effective format.
		f.OutputFormat = format
	}
	if cfg, err := f.Config(); err == nil {
		noColor = noColor || cfg.NoColor
		if format == "" {
			format = cfg.OutputFormat
			if f.OutputFormat == "" {
				f.OutputFormat = format
			}
		}
	}
	if noColor {
		os.Setenv("NO_COLOR", "1")
	}
	return output.NewStdPrinter(f.IOStreams.Out, f.IOStreams.ErrOut, f.IOStreams.IsTTY, noColor, format)
}
