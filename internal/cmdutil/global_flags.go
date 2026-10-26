package cmdutil

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// ApplyGlobalFlags copies the root command's persistent flags (--config,
// --profile, --server, --api-key, --api-key-file, --no-color, --verbose,
// --output, --read-only and --header) onto f. The root command calls it from
// PersistentPreRunE. Completion functions call it too: cobra parses those
// flags for `redmine __complete`, but never runs PersistentPreRunE for the
// command being completed, so without it completion would ignore e.g.
// --config and --profile. Flags the root command does not define are
// skipped. An invalid --header is an error, so a typo fails before any
// interactive prompt.
func ApplyGlobalFlags(f *Factory, cmd *cobra.Command) error {
	flags := cmd.Root().PersistentFlags()
	for name, dst := range map[string]*string{
		"config":       &f.ConfigPath,
		"profile":      &f.ProfileOverride,
		"server":       &f.ServerOverride,
		"api-key":      &f.APIKeyOverride,
		"api-key-file": &f.APIKeyFile,
		"output":       &f.OutputFormat,
	} {
		if v, err := flags.GetString(name); err == nil {
			*dst = v
		}
	}
	if noColor, err := flags.GetBool("no-color"); err == nil && noColor {
		f.NoColorOverride = true
		os.Setenv("NO_COLOR", "1")
	}
	if verbose, err := flags.GetBool("verbose"); err == nil {
		f.Verbose = verbose
	}
	if fl := flags.Lookup("read-only"); fl != nil && fl.Changed {
		readOnly, _ := flags.GetBool("read-only")
		f.ReadOnly = &readOnly
	}
	// The slice is read from the flag value itself: GetStringArray would
	// round-trip it through its CSV string form.
	if fl := flags.Lookup("header"); fl != nil && fl.Changed {
		sv, ok := fl.Value.(interface{ GetSlice() []string })
		if !ok {
			return fmt.Errorf("--header is not a string array flag")
		}
		headers := sv.GetSlice()
		for _, h := range headers {
			if _, _, err := config.ParseHeader(h); err != nil {
				return err
			}
		}
		f.Headers = headers
	}
	return nil
}
