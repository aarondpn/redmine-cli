package extension

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

// Git is the subset of git the Manager needs for script extensions. It is
// an interface so tests can fake clones without a git binary or network.
type Git interface {
	// Clone clones url into dir (which may exist but must be empty). A
	// non-empty ref checks out that tag or branch.
	Clone(ctx context.Context, url, dir, ref string) error
	// Fetch updates the remote-tracking refs of the clone in dir.
	Fetch(ctx context.Context, dir string) error
	// Pull fast-forwards the clone in dir to its upstream.
	Pull(ctx context.Context, dir string) error
	// ResetHard moves the clone in dir to rev, discarding local changes.
	ResetHard(ctx context.Context, dir, rev string) error
	// Revision resolves rev (e.g. HEAD or @{u}) to a commit ID.
	Revision(ctx context.Context, dir, rev string) (string, error)
}

// ErrGitNotFound reports that git is needed but not installed.
var ErrGitNotFound = errors.New("git is required for extensions without a release binary for this platform, but it was not found in PATH")

// ExecGit runs the git binary.
type ExecGit struct{}

func (ExecGit) Clone(ctx context.Context, url, dir, ref string) error {
	args := []string{"-c", "advice.detachedHead=false", "clone", "--quiet"}
	if ref != "" {
		args = append(args, "--depth", "1", "--branch", ref)
	}
	_, err := runGit(ctx, "", append(args, "--", url, dir)...)
	return err
}

func (ExecGit) Fetch(ctx context.Context, dir string) error {
	_, err := runGit(ctx, dir, "fetch", "--quiet")
	return err
}

func (ExecGit) Pull(ctx context.Context, dir string) error {
	_, err := runGit(ctx, dir, "pull", "--ff-only", "--quiet")
	return err
}

func (ExecGit) ResetHard(ctx context.Context, dir, rev string) error {
	_, err := runGit(ctx, dir, "reset", "--hard", "--quiet", rev)
	return err
}

func (ExecGit) Revision(ctx context.Context, dir, rev string) (string, error) {
	out, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// runGit runs git non-interactively: a missing or private repository must
// fail instead of waiting for a credential prompt.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", ErrGitNotFound
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	cmd.Env = append(gitEnv(os.Environ()), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 2048 {
			msg = msg[len(msg)-2048:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s failed: %s", gitSubcommand(args), textsafe.Terminal(msg))
	}
	return stdout.String(), nil
}

// repoLocalGitVars are the variables that point git at a repository other
// than the one in its working directory (`git rev-parse --local-env-vars`
// without the config ones, which CI credential setups rely on, plus
// GIT_CEILING_DIRECTORIES and GIT_NAMESPACE). They are set when
// redmine runs from a git hook or alias, or in dotfile setups that export
// GIT_DIR; inherited, they would make an upgrade's `git reset --hard` act on
// that other repository.
var repoLocalGitVars = map[string]bool{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_CEILING_DIRECTORIES":          true,
	"GIT_COMMON_DIR":                   true,
	"GIT_DIR":                          true,
	"GIT_GRAFT_FILE":                   true,
	"GIT_IMPLICIT_WORK_TREE":           true,
	"GIT_INDEX_FILE":                   true,
	"GIT_INTERNAL_SUPER_PREFIX":        true,
	"GIT_NAMESPACE":                    true,
	"GIT_NO_REPLACE_OBJECTS":           true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_PREFIX":                       true,
	"GIT_REPLACE_REF_BASE":             true,
	"GIT_SHALLOW_FILE":                 true,
	"GIT_WORK_TREE":                    true,
}

// gitEnv returns base without the repository-selecting git variables, so
// git always works on the clone in its working directory.
func gitEnv(base []string) []string {
	out := make([]string, 0, len(base))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if repoLocalGitVars[strings.ToUpper(k)] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// gitSubcommand returns the first non-option argument, for error messages.
func gitSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" {
			i++
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			return args[i]
		}
	}
	return ""
}
