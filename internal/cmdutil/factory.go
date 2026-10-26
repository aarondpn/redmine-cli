package cmdutil

import (
	"io"
	"os"
	"sync"

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

	// APIKeyFile is the --api-key-file path ("-" reads stdin). Its contents
	// are used like APIKeyOverride, without exposing the key in argv.
	APIKeyFile string

	// ReadOnly, when non-nil, is the --read-only flag value and overrides
	// both REDMINE_READ_ONLY and the per-profile config (highest precedence).
	ReadOnly *bool

	// Headers holds the repeatable --header flag values ("Name: value"). They
	// are merged over the profile's configured headers, replacing any entry
	// with the same name.
	Headers []string

	// OutputFormat is set by the root persistent --output/-o flag and used
	// as the default format when commands call Printer("").
	OutputFormat string

	// JQ is the compiled root --jq expression. When set, every JSON
	// document a Printer emits is filtered through it.
	JQ *output.JQFilter

	jqErr error
	// securityMu guards securityWarned, so SecurityCheck can be called from
	// any goroutine.
	securityMu     sync.Mutex
	securityWarned map[string]bool
	apiKeyFromFile string
	config         *config.Config
	client         *api.Client
	debugLog       *debug.Logger
	IOStreams      *IOStreams
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
// CLI flag overrides carry the highest precedence: ServerOverride and
// APIKeyOverride (or the contents of APIKeyFile) are resolved inside Load
// (before any keyring lookup), while NoColorOverride is applied afterward.
func (f *Factory) Config() (*config.Config, error) {
	if f.config != nil {
		return f.config, nil
	}
	apiKey, err := f.apiKeyOverride()
	if err != nil {
		return nil, err
	}
	var cfg *config.Config
	if f.ServerOverride != "" || apiKey != "" {
		// Overrides resolve inside Load, before any keyring lookup.
		cfg, err = config.LoadWithOverrides(f.ConfigPath, f.ProfileOverride, config.Overrides{
			Server: f.ServerOverride,
			APIKey: apiKey,
		}, f.DebugLogger())
	} else {
		cfg, err = config.Load(f.ConfigPath, f.ProfileOverride, f.DebugLogger())
	}
	if err != nil {
		return nil, err
	}

	// A key on the command line is visible in process listings and shell
	// history; strict mode insists on a channel that is not.
	if f.APIKeyOverride != "" && cfg.Strict {
		return nil, &StrictModeError{Msg: "--api-key exposes the API key in process listings and shell history; use --api-key-file, REDMINE_API_KEY, or a keyring profile (redmine auth login --keyring) instead"}
	}

	// A stored secret is bound to the server it was saved for; following a
	// --server / REDMINE_SERVER override elsewhere hands it to another host.
	if profile, server, ok := cfg.StoredCredentialServer(); ok && !config.SameOrigin(server, cfg.Server) {
		if err := f.SecurityCheck(cfg, storedCredentialOverrideMessage(cfg, profile, server)); err != nil {
			return nil, err
		}
	}
	// Profile headers can hold proxy tokens bound to the same server; only
	// those not replaced by a --header on this invocation are checked.
	if profile, server, ok := cfg.ProfileServer(); ok && !config.SameOrigin(server, cfg.Server) {
		if names := profileHeadersInUse(cfg.Headers, f.Headers); len(names) > 0 {
			if err := f.SecurityCheck(cfg, profileHeadersOverrideMessage(cfg, profile, server, names)); err != nil {
				return nil, err
			}
		}
	}

	if f.NoColorOverride {
		cfg.NoColor = true
	}
	if f.ReadOnly != nil {
		cfg.ReadOnly = *f.ReadOnly
	}
	if cfg.Headers, err = config.MergeHeaders(cfg.Headers, f.Headers); err != nil {
		return nil, err
	}

	f.config = cfg
	return cfg, nil
}

// ApiClient returns an API client (cached after first call). A plain http://
// server on a non-loopback host triggers a cleartext-credentials security
// warning, which is an error in strict mode.
func (f *Factory) ApiClient() (*api.Client, error) {
	if f.client != nil {
		return f.client, nil
	}
	cfg, err := f.Config()
	if err != nil {
		return nil, err
	}
	if api.IsCleartextServer(cfg.Server) {
		if err := f.SecurityCheck(cfg, api.CleartextServerWarning(cfg.Server)); err != nil {
			return nil, err
		}
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
	// A config error is surfaced by the command itself; the printer then
	// falls back to the defaults (cfg is nil on error).
	cfg, _ := f.Config()
	if cfg != nil {
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
	strict := cfg != nil && cfg.Strict
	p := output.NewStdPrinter(f.IOStreams.Out, f.IOStreams.ErrOut, f.IOStreams.IsTTY, noColor, format)
	// Strict mode neutralizes CSV formula cells; otherwise they are written
	// verbatim with a one-time warning.
	p.SetCSVFormulaGuard(strict, func(msg string) {
		_ = f.SecurityCheck(cfg, msg)
	})
	if f.JQ != nil && p.Format() == output.FormatJSON {
		q := f.JQ
		if strict {
			q = q.WithoutEnv()
		}
		p.SetJQ(q, f.recordJQError)
	}
	return p
}

// JQError returns the first runtime error raised while applying --jq to a
// command's output, or nil.
func (f *Factory) JQError() error {
	return f.jqErr
}

func (f *Factory) recordJQError(err error) {
	if f.jqErr == nil {
		f.jqErr = err
	}
}
