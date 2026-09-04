//go:build !windows

package processrun

import (
	"os/exec"
	"syscall"
)

func signalExitCode(exitError *exec.ExitError) (int, bool) {
	status, ok := exitError.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return 0, false
	}
	return 128 + int(status.Signal()), true
}
