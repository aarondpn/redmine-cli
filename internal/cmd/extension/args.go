package extension

import (
	"context"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type invocationArgsKey struct{}

// WithInvocationArgs records the arguments the root command is executed
// with (what it was given with SetArgs). Passthrough commands need them to
// tell redmine-cli's own flags in front of the command apart from the
// extension's arguments; see passthroughArgs. Without them the process
// arguments are used, which is what cobra executes when SetArgs was not
// called.
func WithInvocationArgs(ctx context.Context, args []string) context.Context {
	return context.WithValue(ctx, invocationArgsKey{}, slices.Clone(args))
}

func invocationArgs(ctx context.Context) []string {
	if ctx != nil {
		if args, ok := ctx.Value(invocationArgsKey{}).([]string); ok {
			return args
		}
	}
	if len(os.Args) < 2 {
		return nil
	}
	return os.Args[1:]
}

// passthroughArgs splits the arguments cobra hands a passthrough command
// into the redmine-cli global flags and the arguments for the extension.
//
// Cobra does not parse flags of a DisableFlagParsing command, and it gives
// it the flags in front of the command path and the arguments after it as
// one list, so `redmine --profile a demo sh --profile b` and `redmine demo
// sh --profile a --profile b` look the same. The split is therefore made on
// the invocation arguments: everything in front of (and between the names
// of) the command path is redmine-cli's, everything after it belongs to the
// extension unchanged, even when it looks like a global flag.
//
// When the invocation arguments do not lead to args (a caller that used
// SetArgs without WithInvocationArgs), it falls back to taking the known
// global flags at the start of args.
func passthroughArgs(cmd *cobra.Command, args []string) (global, rest []string) {
	before, after, ok := splitAtCommandPath(cmd, invocationArgs(cmd.Context()))
	if ok && slices.Equal(append(slices.Clone(before), after...), args) {
		return before, after
	}
	return splitGlobalFlags(cmd.Root().PersistentFlags(), args)
}

// splitAtCommandPath finds the names of cmd's command path in raw the way
// cobra's Find does (the first argument that is neither a flag nor a flag
// value names the next subcommand) and returns the arguments in front of
// and between those names, and the ones after the last. ok is false when
// raw does not lead to cmd.
func splitAtCommandPath(cmd *cobra.Command, raw []string) (before, after []string, ok bool) {
	var path []*cobra.Command
	for c := cmd; c.HasParent(); c = c.Parent() {
		path = append([]*cobra.Command{c}, path...)
	}
	parent := cmd.Root()
	i := 0
	for _, c := range path {
		j := nextCommandArg(parent, raw, i)
		if j < 0 || (raw[j] != c.Name() && !c.HasAlias(raw[j])) {
			return nil, nil, false
		}
		before = append(before, raw[i:j]...)
		i = j + 1
		parent = c
	}
	return before, raw[i:], true
}

// nextCommandArg returns the index of the first argument at or after from
// that is not a flag of parent or the value of one, or -1.
func nextCommandArg(parent *cobra.Command, raw []string, from int) int {
	for i := from; i < len(raw); i++ {
		s := raw[i]
		switch {
		case s == "--":
			return -1
		case strings.HasPrefix(s, "--") && !strings.Contains(s, "="):
			if !noOptDefVal(lookupFlag(parent, s[2:])) {
				i++ // the flag's value
			}
		case strings.HasPrefix(s, "-") && len(s) == 2:
			if !noOptDefVal(lookupShorthand(parent, s[1:])) {
				i++
			}
		case strings.HasPrefix(s, "-"):
		default:
			return i
		}
	}
	return -1
}

func noOptDefVal(f *pflag.Flag) bool {
	return f != nil && f.NoOptDefVal != ""
}

func lookupFlag(c *cobra.Command, name string) *pflag.Flag {
	if f := c.Flags().Lookup(name); f != nil {
		return f
	}
	return c.InheritedFlags().Lookup(name)
}

func lookupShorthand(c *cobra.Command, name string) *pflag.Flag {
	if f := c.Flags().ShorthandLookup(name); f != nil {
		return f
	}
	return c.InheritedFlags().ShorthandLookup(name)
}
