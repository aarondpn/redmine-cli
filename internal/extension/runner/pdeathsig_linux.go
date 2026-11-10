package runner

import "syscall"

// setParentDeathSignal makes Linux kill the process when redmine dies
// without getting to kill its process group (SIGKILL, a crash). It only
// reaches the process itself, not what it started; the group kill on
// cancellation stays the main mechanism. (The signal follows the thread
// that started the process; the Go runtime only ends threads locked with
// runtime.LockOSThread, which redmine does not use around runs.)
func setParentDeathSignal(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGKILL
}
