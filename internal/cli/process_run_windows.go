//go:build windows

package cli

import "os/exec"

func runProcess(command *exec.Cmd) error {
	return command.Run()
}
