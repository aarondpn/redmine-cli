package extension

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func setStderrTerminal(t *testing.T, tty bool) {
	t.Helper()
	orig := stderrIsTerminal
	stderrIsTerminal = func(*cmdutil.Factory) bool { return tty }
	t.Cleanup(func() { stderrIsTerminal = orig })
}

func TestUpdateNotice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake binary is a shell script")
	}
	tests := []struct {
		name       string
		tty        bool
		noCheck    string
		pin        bool
		wantNotice bool
	}{
		{name: "new release at a terminal", tty: true, wantNotice: true},
		{name: "not a terminal", tty: false},
		{name: "disabled", tty: true, noCheck: "1"},
		{name: "pinned", tty: true, pin: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			setStderrTerminal(t, tt.tty)
			t.Setenv("REDMINE_NO_UPDATE_CHECK", tt.noCheck)
			asset := extmgr.BinaryAssetName("pr", "linux", "amd64")
			te.gh.set("octo/redmine-cli-pr", "v1.0.0", map[string][]byte{asset: []byte("#!/bin/sh\nexit 0\n")})
			if _, err := te.manager().Install(t.Context(), extmgr.Source{Owner: "octo", Repo: "redmine-cli-pr", Name: "pr"}, extmgr.InstallOptions{}); err != nil {
				t.Fatal(err)
			}
			if tt.pin {
				markPinned(t, te)
			}
			te.gh.set("octo/redmine-cli-pr", "v1.1.0", map[string][]byte{asset: []byte("new")})

			if err := te.run("extension", "exec", "pr"); err != nil {
				t.Fatalf("%v\n%s", err, te.stderr)
			}
			got := strings.Contains(te.stderr.String(), "A new release of pr is available: v1.0.0 → v1.1.0")
			if got != tt.wantNotice {
				t.Fatalf("notice shown = %v, want %v; stderr: %q", got, tt.wantNotice, te.stderr)
			}
			if tt.wantNotice && !strings.Contains(te.stderr.String(), `redmine extension upgrade pr`) {
				t.Errorf("notice lacks the upgrade command: %q", te.stderr)
			}
		})
	}
}

// markPinned rewrites the install metadata as if installed with --pin.
func markPinned(t *testing.T, te *testEnv) {
	t.Helper()
	ext, err := te.manager().Get("pr")
	if err != nil {
		t.Fatal(err)
	}
	ext.Install.Pinned = true
	if err := writeYAML(filepath.Join(ext.Dir, extmgr.InstallFile), ext.Install); err != nil {
		t.Fatal(err)
	}
}

func TestDestructiveConfirmation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture extension is a shell script")
	}
	tests := []struct {
		name     string
		answer   string
		wantRan  bool
		wantText string
	}{
		{name: "confirmed", answer: "y\n", wantRan: true},
		{name: "declined", answer: "n\n", wantText: "Cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			copyFixture(t, filepath.Join(te.extRoot, "redmine-cli-fixture"))
			setTerminal(t, true)
			te.stdin.WriteString(tt.answer)
			if err := te.run("extension", "exec", "fixture", "wipe"); err != nil {
				t.Fatalf("%v\n%s", err, te.stderr)
			}
			if !strings.Contains(te.stderr.String(), "redmine fixture wipe is destructive. Continue? [y/N]") {
				t.Errorf("prompt missing: %q", te.stderr)
			}
			ran := strings.Contains(te.stdout.String(), `"REDMINE_EXT_COMMAND"`)
			if ran != tt.wantRan {
				t.Fatalf("ran = %v, want %v; stdout %q", ran, tt.wantRan, te.stdout)
			}
			if tt.wantText != "" && !strings.Contains(te.stderr.String(), tt.wantText) {
				t.Errorf("stderr = %q, want %q", te.stderr, tt.wantText)
			}
		})
	}
}

func TestRegisterSkipsCoreNamesAndGroups(t *testing.T) {
	root := &cobra.Command{Use: "redmine"}
	root.AddCommand(&cobra.Command{Use: "issues", Aliases: []string{"issue"}, Run: func(*cobra.Command, []string) {}})
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{}}
	exts := []*extmgr.Extension{
		{Name: "issue", Kind: extmgr.KindLocal},
		{Name: "pr", Kind: extmgr.KindLocal},
		{Name: "pr", Kind: extmgr.KindLocal},
	}
	added := Register(root, f, exts, Options{})
	if len(added) != 1 || added[0].Name() != "pr" || added[0].GroupID != groupExtensions {
		t.Fatalf("added = %v", added)
	}
	for _, c := range root.Commands() {
		if c.Name() == "issues" && c.GroupID != groupCore {
			t.Errorf("core command group = %q", c.GroupID)
		}
	}
	if !IsExtensionCommand(added[0]) {
		t.Error("extension command not annotated")
	}
	if names := CoreCommandNames(root); strings.Join(names, ",") != "issues,issue" {
		t.Errorf("core names = %v", names)
	}

	empty := &cobra.Command{Use: "redmine"}
	if Register(empty, f, nil, Options{}) != nil || len(empty.Groups()) != 0 {
		t.Error("no extensions must leave the root untouched")
	}
}

func TestBrokenErrorCarriesDetails(t *testing.T) {
	te := newTestEnv(t)
	dir := filepath.Join(te.extRoot, "redmine-cli-fixture")
	copyFixture(t, dir)
	writeFile(t, filepath.Join(dir, "redmine-extension.yml"), "schema_version: 1\nname: fixture\n")
	err := te.run("extension", "exec", "fixture", "show", "1")
	wantCode(t, err, output.ErrCodeValidationFailed)
	if hint := cmdutil.ErrorHint(err); !strings.Contains(hint, "redmine extension upgrade fixture") {
		t.Errorf("hint = %q", hint)
	}
}

// TestHelpTextsAreSanitized builds the commands of an extension whose
// manifest texts carry escape sequences (validation rejects them; this is
// the second line of defense): help, usage and completion descriptions
// must not hand them to the terminal.
func TestHelpTextsAreSanitized(t *testing.T) {
	const evil = "go \x1b[31mRED\x1b]52;c;eA==\x07"
	ext := &extmgr.Extension{Name: "ro", Kind: extmgr.KindLocal, Manifest: &spec.Manifest{
		SchemaVersion: 1, Name: "ro", Description: evil,
		Commands: []spec.Command{
			{Name: "show", Description: evil, Params: []spec.Param{{Name: "state", Type: spec.TypeString, Description: evil}}},
			{Name: "shell", Description: evil, Raw: true},
		},
	}}
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{}}
	parent := NewExtensionCommand(f, ext, Options{})
	texts := []string{parent.Short, parent.Long, describe(ext)}
	for _, c := range parent.Commands() {
		texts = append(texts, c.Short, c.Long, c.UsageString())
	}
	for _, s := range texts {
		if strings.ContainsRune(s, 0x1b) || strings.ContainsRune(s, 0x07) {
			t.Errorf("raw control characters in %q", s)
		}
	}
	if !strings.Contains(parent.Short, "go ^[[31mRED") {
		t.Errorf("Short = %q, want the text neutralized", parent.Short)
	}
}
