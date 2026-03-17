package cmdutil

import (
	"io"
	"os"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/output"
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
	config     *config.Config
	client     *api.Client
	printer    output.Printer
	IOStreams  *IOStreams
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

// Config returns the loaded configuration (cached after first call).
func (f *Factory) Config() (*config.Config, error) {
	if f.config != nil {
		return f.config, nil
	}
	cfg, err := config.Load(f.ConfigPath)
	if err != nil {
		return nil, err
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
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	f.client = client
	return client, nil
}

// Printer returns the output printer.
func (f *Factory) Printer(format string) output.Printer {
	if f.printer != nil {
		return f.printer
	}
	noColor := false
	if cfg, err := f.Config(); err == nil {
		noColor = cfg.NoColor
	}
	f.printer = output.NewStdPrinter(f.IOStreams.Out, f.IOStreams.ErrOut, f.IOStreams.IsTTY, noColor, format)
	return f.printer
}
