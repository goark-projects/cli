//go:build !windows

package processrun

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func run(ctx context.Context, command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return err
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	for {
		select {
		case err := <-waited:
			return err
		case received := <-signals:
			_ = syscall.Kill(-command.Process.Pid, received.(syscall.Signal))
		case <-ctx.Done():
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-waited:
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
				<-waited
			}
			return ctx.Err()
		}
	}
}
