//go:build !windows

package cli

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

type readinessWriter struct {
	ready chan struct{}
	once  sync.Once
}

func (w *readinessWriter) Write(data []byte) (int, error) {
	w.once.Do(func() { close(w.ready) })
	return len(data), nil
}

func TestOSProcessRunner_whenParentReceivesInterrupt_shouldForwardSignalToChild(t *testing.T) {
	result := make(chan error, 1)
	output := &readinessWriter{ready: make(chan struct{})}
	go func() {
		result <- osProcessRunner{}.Run(ProcessRequest{
			Name: "sh",
			Args: []string{"-c", `trap 'exit 23' INT; echo ready; while :; do sleep 1; done`},
			Out:  output,
			Err:  io.Discard,
		})
	}()
	select {
	case <-output.ready:
	case <-time.After(3 * time.Second):
		t.Fatal("等待子进程启动超时")
	}
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("查找当前进程失败: %v", err)
	}
	if err := process.Signal(os.Interrupt); err != nil {
		t.Fatalf("发送中断信号失败: %v", err)
	}

	select {
	case runErr := <-result:
		var exitError *exec.ExitError
		if !errors.As(runErr, &exitError) || exitError.ExitCode() != 23 {
			t.Fatalf("子进程未收到中断信号: %v", runErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("等待子进程处理中断信号超时")
	}
}
