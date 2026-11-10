//go:build windows

package runner

import (
	"os"
	"os/exec"
)

// startInGroup is a no-op on Windows: there is no process group to kill,
// killGroup ends the process itself.
func startInGroup(cmd *exec.Cmd) {}

// killGroup kills the process.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// exitCode returns the process exit code, 1 when it is unknown.
func exitCode(ps *os.ProcessState) int {
	if ps == nil {
		return 1
	}
	if code := ps.ExitCode(); code > 0 {
		return code
	}
	return 1
}
