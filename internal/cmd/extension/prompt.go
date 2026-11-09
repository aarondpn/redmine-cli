package extension

import (
	"os"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// stdinIsTerminal reports whether a [y/N] prompt can be answered. It is a
// variable so tests can simulate a terminal.
var stdinIsTerminal = func(f *cmdutil.Factory) bool {
	file, ok := f.IOStreams.In.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// stderrIsTerminal gates the extension update notice, which is only for
// people at a terminal. It is a variable for tests.
var stderrIsTerminal = func(f *cmdutil.Factory) bool {
	file, ok := f.IOStreams.ErrOut.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// splitGlobalFlags splits arguments whose flags cobra does not parse into
// the leading redmine-cli global flags and the rest. Parsing stops at the
// first argument that is not a known global flag. `extension exec` uses it
// for the flags in front of the extension name; passthrough commands use
// passthroughArgs, which knows where the command path ended.
func splitGlobalFlags(fs *pflag.FlagSet, args []string) (global, rest []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" || a == "-" || !strings.HasPrefix(a, "-") {
			break
		}
		var (
			fl     *pflag.Flag
			inline bool
		)
		if name, ok := strings.CutPrefix(a, "--"); ok {
			name, _, inline = strings.Cut(name, "=")
			fl = fs.Lookup(name)
		} else {
			fl = fs.ShorthandLookup(a[1:2])
			inline = len(a) > 2
			// "-vx" combines shorthands, which only bool flags allow; leave
			// such arguments to the extension rather than guess.
			if fl != nil && fl.NoOptDefVal != "" && inline && a[2] != '=' {
				break
			}
		}
		if fl == nil {
			break
		}
		n := 1
		if !inline && fl.NoOptDefVal == "" {
			n = 2
		}
		if i+n > len(args) {
			break
		}
		global = append(global, args[i:i+n]...)
		i += n
	}
	return global, args[i:]
}
