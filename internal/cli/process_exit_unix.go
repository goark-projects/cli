//go:build !windows

package cli

import "goark.dev/cli/internal/processrun"

func signaledProcessExitCode(err error) (int, bool) {
	return processrun.ExitCode(err)
}
