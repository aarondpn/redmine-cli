//go:build !windows

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// startInGroup puts the process into a new process group, so killGroup
// also reaches everything it started (e.g. a "redmine api" call or a
// sleeping subshell).
func startInGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup kills the process group of cmd.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// exitCode follows the shell convention for a process killed by a signal
// (128 + signal number).
func exitCode(ps *os.ProcessState) int {
	if ps == nil {
		return 1
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	if code := ps.ExitCode(); code > 0 {
		return code
	}
	return 1
}
