package processrun

import (
	"errors"
	"os/exec"
)

// ExitCode 提取已启动进程的原始退出码。
func ExitCode(err error) (int, bool) {
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		var generic interface{ ExitCode() int }
		if errors.As(err, &generic) && generic.ExitCode() >= 0 {
			return generic.ExitCode(), true
		}
		return 0, false
	}
	if code := exitError.ExitCode(); code >= 0 {
		return code, true
	}
	return signalExitCode(exitError)
}
