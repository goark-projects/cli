//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

func runProcess(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		return err
	}
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		for {
			select {
			case received := <-signals:
				_ = command.Process.Signal(received)
			case <-done:
				return
			}
		}
	}()
	err := command.Wait()
	signal.Stop(signals)
	close(done)
	group.Wait()
	return err
}
