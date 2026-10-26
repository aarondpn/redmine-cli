//go:build !windows

package completion

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// completionScript generates the completion script for shell from a minimal
// "redmine" command tree with one flag that has dynamic completions.
func completionScript(t *testing.T, shell string) string {
	t.Helper()
	root := &cobra.Command{Use: "redmine"}
	issues := &cobra.Command{Use: "issues", Run: func(*cobra.Command, []string) {}}
	issues.Flags().String("tracker", "", "")
	_ = issues.RegisterFlagCompletionFunc("tracker", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
	root.AddCommand(issues, NewCmdCompletion())

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"completion", shell})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "redmine."+shell)
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeRedmine puts a "redmine" executable on PATH that answers every
// __complete request with candidates that are shell code (as a tracker,
// version or member name on the server could be), plus one plain candidate.
// It returns the marker file the shell code would create.
func fakeRedmine(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' 'plain-candidate' '$(touch${IFS}" + marker + ")' '`touch${IFS}" + marker + "`' '$(touch${IFS}" + marker + ")\tdescription' ':4'\n"
	if err := os.WriteFile(filepath.Join(dir, "redmine"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

// TestCompletionScripts_DoNotEvaluateCandidates runs the generated bash and
// zsh scripts for real (when the shell is installed) and checks that
// completion candidates are never evaluated as shell code.
func TestCompletionScripts_DoNotEvaluateCandidates(t *testing.T) {
	tests := []struct {
		shell string
		// driver sources the script at $1 and triggers completion of
		// "redmine issues --tracker <cur>", printing the resulting
		// candidates.
		driver string
	}{
		{"bash", `
if ! declare -F _get_comp_words_by_ref >/dev/null; then
  # Minimal stand-in for the bash-completion helper the script uses.
  _get_comp_words_by_ref() {
    cur=${COMP_WORDS[COMP_CWORD]}; prev=${COMP_WORDS[COMP_CWORD-1]}
    words=("${COMP_WORDS[@]}"); cword=$COMP_CWORD
  }
fi
source "$1"
for cur in "" '$(' '` + "`" + `'; do
  COMP_WORDS=(redmine issues --tracker "$cur"); COMP_CWORD=3
  COMP_LINE="redmine issues --tracker $cur"; COMP_POINT=${#COMP_LINE}
  COMP_TYPE=9; COMP_KEY=9; COMPREPLY=()
  __start_redmine
  printf '%s\n' "${COMPREPLY[@]}"
done
`},
		{"zsh", `
compdef() { :; }
compadd() { :; }
_arguments() { :; }
_describe() { print -rl -- "${completions[@]}"; }
source "$1"
for cur in "" '$(' '` + "`" + `'; do
  words=(redmine issues --tracker "$cur"); CURRENT=4
  _redmine
done
`},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			shell, err := exec.LookPath(tt.shell)
			if err != nil {
				t.Skipf("%s not installed", tt.shell)
			}
			script := completionScript(t, tt.shell)
			marker := fakeRedmine(t)

			args := []string{"-c", tt.driver, tt.shell, script}
			if tt.shell == "bash" {
				args = append([]string{"--norc", "--noprofile"}, args...)
			} else {
				args = append([]string{"-f"}, args...)
			}
			out, err := exec.Command(shell, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("%s failed: %v\n%s", tt.shell, err, out)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("%s completion executed a candidate; output:\n%s", tt.shell, out)
			}
			// The fake binary was really consulted.
			if !strings.Contains(string(out), "plain-candidate") {
				t.Fatalf("%s completion did not offer the plain candidate; output:\n%s", tt.shell, out)
			}
		})
	}
}
