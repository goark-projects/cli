//go:build windows

package cli

func signaledProcessExitCode(error) (int, bool) {
	return 0, false
}
