//go:build windows

package update

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestHelperProcessSleep is not a real test: the running-binary tests start a
// copy of the test binary with it so that redmine.exe is a live process.
func TestHelperProcessSleep(t *testing.T) {
	if os.Getenv("REDMINE_UPDATE_HELPER_SLEEP") != "1" {
		t.Skip("helper process")
	}
	time.Sleep(time.Minute)
}

// startRunningExecutable copies the test binary to dir/redmine.exe and runs it
// until the test ends, so the file is locked the way a running redmine.exe is
// during `redmine update`.
func startRunningExecutable(t *testing.T, dir string) (string, *exec.Cmd) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "redmine.exe")
	copyExecutable(t, self, exe)

	cmd := exec.Command(exe, "-test.run=^TestHelperProcessSleep$")
	cmd.Env = append(os.Environ(), "REDMINE_UPDATE_HELPER_SLEEP=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return exe, cmd
}

func copyExecutable(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceBinary_RunningExecutable(t *testing.T) {
	if !moveRunningBinaryAside {
		t.Fatal("moveRunningBinaryAside must be true on Windows")
	}
	dir := t.TempDir()
	exe, cmd := startRunningExecutable(t, dir)

	// The reason for moving aside: a plain rename onto the running binary
	// is expected to be refused. Logged rather than asserted, since the swap
	// below must work either way.
	probe := filepath.Join(dir, ".redmine-probe")
	writeFile(t, probe, "probe")
	if err := os.Rename(probe, exe); err != nil {
		t.Logf("overwriting the running executable fails as expected: %v", err)
	} else {
		t.Log("overwriting the running executable succeeded; moving aside is not required on this Windows version")
	}
	os.Remove(probe)

	if err := replaceBinary(exe, []byte("new")); err != nil {
		t.Fatalf("replaceBinary while redmine.exe runs: %v", err)
	}
	if got := readFile(t, exe); got != "new" {
		t.Errorf("redmine.exe content = %q, want %q", got, "new")
	}
	if _, err := os.Stat(exe + ".old"); err != nil {
		t.Errorf("expected the running binary at redmine.exe.old: %v", err)
	}

	// Once the old process has exited, the next update removes its .old file.
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if err := replaceBinary(exe, []byte("newer")); err != nil {
		t.Fatalf("second replaceBinary: %v", err)
	}
	if got := readFile(t, exe); got != "newer" {
		t.Errorf("redmine.exe content = %q, want %q", got, "newer")
	}
	if got := readFile(t, exe+".old"); got != "new" {
		t.Errorf("redmine.exe.old content = %q, want the previous %q", got, "new")
	}
}
