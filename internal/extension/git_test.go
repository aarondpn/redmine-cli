package extension

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitCmd runs git in dir with a fixed identity and no user or system
// config, so the test does not depend on the machine's git setup.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestExecGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ctx := context.Background()
	upstream := filepath.Join(t.TempDir(), "upstream")
	if err := os.MkdirAll(upstream, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, upstream, "init", "--quiet")
	if err := os.WriteFile(filepath.Join(upstream, "redmine-cli-scr"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, upstream, "add", ".")
	gitCmd(t, upstream, "commit", "--quiet", "-m", "one")
	gitCmd(t, upstream, "tag", "v1.0.0")
	first := gitCmd(t, upstream, "rev-parse", "HEAD")

	g := ExecGit{}
	url := "file://" + filepath.ToSlash(upstream)
	clone := filepath.Join(t.TempDir(), "clone")
	if err := g.Clone(ctx, url, clone, ""); err != nil {
		t.Fatal(err)
	}
	if rev, err := g.Revision(ctx, clone, "HEAD"); err != nil || rev != first {
		t.Fatalf("HEAD = %q, %v; want %s", rev, err, first)
	}
	if rev, err := readGitHead(clone); err != nil || rev != first {
		t.Fatalf("readGitHead = %q, %v", rev, err)
	}

	if err := os.WriteFile(filepath.Join(upstream, "README.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, upstream, "add", ".")
	gitCmd(t, upstream, "commit", "--quiet", "-m", "two")
	second := gitCmd(t, upstream, "rev-parse", "HEAD")

	if err := g.Fetch(ctx, clone); err != nil {
		t.Fatal(err)
	}
	if rev, err := g.Revision(ctx, clone, "@{u}"); err != nil || rev != second {
		t.Fatalf("@{u} = %q, %v; want %s", rev, err, second)
	}
	if err := g.Pull(ctx, clone); err != nil {
		t.Fatal(err)
	}
	if rev, _ := g.Revision(ctx, clone, "HEAD"); rev != second {
		t.Fatalf("HEAD after pull = %s, want %s", rev, second)
	}
	if err := os.WriteFile(filepath.Join(clone, "README.md"), []byte("local change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.ResetHard(ctx, clone, "@{u}"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(clone, "README.md")); string(data) != "two\n" {
		t.Fatalf("reset kept the local change: %q", data)
	}

	pinned := filepath.Join(t.TempDir(), "pinned")
	if err := g.Clone(ctx, url, pinned, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if rev, _ := g.Revision(ctx, pinned, "HEAD"); rev != first {
		t.Fatalf("pinned HEAD = %s, want %s", rev, first)
	}

	err := g.Clone(ctx, "file://"+filepath.ToSlash(filepath.Join(t.TempDir(), "missing")), filepath.Join(t.TempDir(), "x"), "")
	if err == nil || !strings.Contains(err.Error(), "git clone failed") {
		t.Fatalf("missing repository: %v", err)
	}
}

// TestExecGitIgnoresInheritedRepository runs every git operation with
// GIT_DIR and GIT_WORK_TREE pointing at another repository, as in a git
// hook: git must still work on the extension clone and leave the other
// repository alone.
func TestExecGitIgnoresInheritedRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ctx := context.Background()

	newRepo := func(name, file string) (dir, head string) {
		dir = filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "init", "--quiet")
		if err := os.WriteFile(filepath.Join(dir, file), []byte(name+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "commit", "--quiet", "-m", name)
		return dir, gitCmd(t, dir, "rev-parse", "HEAD")
	}
	upstream, first := newRepo("upstream", "redmine-cli-scr")
	other, otherHead := newRepo("other", "precious.txt")
	if err := os.WriteFile(filepath.Join(other, "precious.txt"), []byte("uncommitted work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(other, ".git", "index"))

	g := ExecGit{}
	clone := filepath.Join(t.TempDir(), "clone")
	if err := g.Clone(ctx, "file://"+filepath.ToSlash(upstream), clone, ""); err != nil {
		t.Fatal(err)
	}
	if rev, err := readGitHead(clone); err != nil || rev != first {
		t.Fatalf("clone HEAD = %q, %v; want %s (the clone went elsewhere)", rev, err, first)
	}
	if rev, err := g.Revision(ctx, clone, "HEAD"); err != nil || rev != first {
		t.Fatalf("Revision(HEAD) = %q, %v; want %s", rev, err, first)
	}
	if err := g.Fetch(ctx, clone); err != nil {
		t.Fatal(err)
	}
	if err := g.Pull(ctx, clone); err != nil {
		t.Fatal(err)
	}
	if err := g.ResetHard(ctx, clone, "@{u}"); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		_ = os.Unsetenv(k)
	}
	if data, _ := os.ReadFile(filepath.Join(other, "precious.txt")); string(data) != "uncommitted work\n" {
		t.Fatalf("the other repository's work tree was changed: %q", data)
	}
	if head := gitCmd(t, other, "rev-parse", "HEAD"); head != otherHead {
		t.Fatalf("the other repository's HEAD moved to %s", head)
	}
	if _, err := os.Stat(filepath.Join(clone, "precious.txt")); err == nil {
		t.Fatal("the clone got the other repository's files")
	}
}

func TestGitEnvDropsRepositoryVariables(t *testing.T) {
	got := gitEnv([]string{"GIT_DIR=/x", "PATH=/bin", "GIT_WORK_TREE=/y", "GIT_CONFIG_COUNT=1", "git_dir=/z", "GIT_TERMINAL_PROMPT=1", "GIT_COMMON_DIR=/c"})
	want := []string{"PATH=/bin", "GIT_CONFIG_COUNT=1", "GIT_TERMINAL_PROMPT=1"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("gitEnv = %v, want %v", got, want)
	}
}
