package extension

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// argsTree is a root with global flags, a manifest extension "demo" with
// a raw command "sh", and a manifest-less extension "plain".
func argsTree() (root, demo, sh, plain *cobra.Command) {
	root = &cobra.Command{Use: "redmine"}
	root.PersistentFlags().String("profile", "", "")
	root.PersistentFlags().StringP("output", "o", "", "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().Bool("read-only", false, "")
	demo = &cobra.Command{Use: "demo"}
	sh = &cobra.Command{Use: "sh", DisableFlagParsing: true, Run: func(*cobra.Command, []string) {}}
	plain = &cobra.Command{Use: "plain", DisableFlagParsing: true, Run: func(*cobra.Command, []string) {}}
	demo.AddCommand(sh)
	root.AddCommand(demo, plain)
	return root, demo, sh, plain
}

func TestSplitAtCommandPath(t *testing.T) {
	_, _, sh, plain := argsTree()
	tests := []struct {
		name       string
		cmd        *cobra.Command
		raw        string
		wantBefore string
		wantAfter  string
		wantOK     bool
	}{
		{name: "nothing around", cmd: sh, raw: "demo sh", wantOK: true},
		{name: "flags after stay", cmd: sh, raw: "demo sh -v --output x a", wantAfter: "-v --output x a", wantOK: true},
		{name: "flags before are global", cmd: sh, raw: "--profile p1 demo sh --profile x a", wantBefore: "--profile p1", wantAfter: "--profile x a", wantOK: true},
		{name: "flags between path names", cmd: sh, raw: "-v demo -o json sh x", wantBefore: "-v -o json", wantAfter: "x", wantOK: true},
		{name: "flag value equal to a command name", cmd: sh, raw: "--profile demo demo sh", wantBefore: "--profile demo", wantOK: true},
		{name: "bool flag does not take the name", cmd: sh, raw: "--read-only demo sh", wantBefore: "--read-only", wantOK: true},
		{name: "inline values", cmd: sh, raw: "--profile=p -o=json demo sh -v", wantBefore: "--profile=p -o=json", wantAfter: "-v", wantOK: true},
		{name: "manifest-less", cmd: plain, raw: "-v plain -v --profile x", wantBefore: "-v", wantAfter: "-v --profile x", wantOK: true},
		{name: "other command", cmd: plain, raw: "demo sh", wantOK: false},
		{name: "double dash before the path", cmd: plain, raw: "-- plain", wantOK: false},
		{name: "empty", cmd: plain, raw: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, after, ok := splitAtCommandPath(tt.cmd, strings.Fields(tt.raw))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if strings.Join(before, " ") != tt.wantBefore || strings.Join(after, " ") != tt.wantAfter {
				t.Fatalf("before %q after %q, want %q / %q", before, after, tt.wantBefore, tt.wantAfter)
			}
		})
	}
}

// TestPassthroughArgsAgreesWithCobra runs the tree through cobra and checks
// that the split of what cobra hands the passthrough command matches the
// position of the command in the invocation.
func TestPassthroughArgsAgreesWithCobra(t *testing.T) {
	tests := []struct {
		raw        string
		wantGlobal string
		wantRest   string
	}{
		{raw: "demo sh -v --output x a", wantRest: "-v --output x a"},
		{raw: "--profile p1 demo sh --profile x a", wantGlobal: "--profile p1", wantRest: "--profile x a"},
		{raw: "-v demo --profile p sh -h", wantGlobal: "-v --profile p", wantRest: "-h"},
		{raw: "plain --profile x", wantRest: "--profile x"},
		{raw: "-o json plain sub", wantGlobal: "-o json", wantRest: "sub"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			root, _, sh, plain := argsTree()
			var global, rest []string
			capture := func(cmd *cobra.Command, args []string) { global, rest = passthroughArgs(cmd, args) }
			sh.Run, plain.Run = capture, capture
			raw := strings.Fields(tt.raw)
			root.SetArgs(raw)
			if err := root.ExecuteContext(WithInvocationArgs(context.Background(), raw)); err != nil {
				t.Fatal(err)
			}
			if strings.Join(global, " ") != tt.wantGlobal || strings.Join(rest, " ") != tt.wantRest {
				t.Fatalf("global %q rest %q, want %q / %q", global, rest, tt.wantGlobal, tt.wantRest)
			}
		})
	}
}

// TestPassthroughArgsFallback covers a root executed with SetArgs but
// without WithInvocationArgs: the process arguments do not lead to the
// command, so only the known global flags at the start count.
func TestPassthroughArgsFallback(t *testing.T) {
	root, _, sh, _ := argsTree()
	var global, rest []string
	sh.Run = func(cmd *cobra.Command, args []string) { global, rest = passthroughArgs(cmd, args) }
	root.SetArgs([]string{"--profile", "p", "demo", "sh", "x", "-v"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(global, []string{"--profile", "p"}) || !slices.Equal(rest, []string{"x", "-v"}) {
		t.Fatalf("global %q rest %q", global, rest)
	}
}
