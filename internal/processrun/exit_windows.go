//go:build windows

package processrun

import "os/exec"

func signalExitCode(*exec.ExitError) (int, bool) {
	return 0, false
}
