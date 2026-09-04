//go:build !windows

package cli

import (
	"os/exec"
	"testing"
)

func TestSignaledProcessExitCode_whenChildReceivesTerm_shouldReturnShellExitCode(t *testing.T) {
	err := exec.Command("sh", "-c", "kill -TERM $$").Run()
	code, ok := signaledProcessExitCode(err)
	if !ok || code != 143 {
		t.Fatalf("信号退出码 = %d, %v, error=%v", code, ok, err)
	}
}
