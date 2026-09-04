package cli

import (
	"io"
	"os"
	"os/exec"
)

// ProcessRequest 描述一个需要继承终端边界的子进程。
type ProcessRequest struct {
	Name string
	Args []string
	Dir  string
	Env  []string
	In   io.Reader
	Out  io.Writer
	Err  io.Writer
}

// ProcessRunner 隔离操作系统进程执行，便于验证参数透传和退出语义。
type ProcessRunner interface {
	Run(request ProcessRequest) error
}

type osProcessRunner struct{}

func (osProcessRunner) Run(request ProcessRequest) error {
	command := exec.Command(request.Name, request.Args...)
	command.Dir = request.Dir
	command.Env = request.Env
	if len(command.Env) == 0 {
		command.Env = os.Environ()
	}
	command.Stdin = request.In
	if command.Stdin == nil {
		command.Stdin = os.Stdin
	}
	command.Stdout = request.Out
	command.Stderr = request.Err
	return runProcess(command)
}
