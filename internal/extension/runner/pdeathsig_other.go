//go:build !windows && !linux

package runner

import "syscall"

// setParentDeathSignal is Linux only.
func setParentDeathSignal(*syscall.SysProcAttr) {}
