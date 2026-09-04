package processrun

import (
	"context"
	"io"
	"os"
	"os/exec"
)

// Request 描述一个需要继承终端边界的子进程。
type Request struct {
	Context context.Context
	Name    string
	Args    []string
	Dir     string
	Env     []string
	In      io.Reader
	Out     io.Writer
	Err     io.Writer
}

// Runner 隔离操作系统进程执行。
type Runner interface {
	Run(Request) error
}

// OSRunner 使用当前操作系统的进程树和信号语义执行请求。
type OSRunner struct{}

// Run 启动进程并等待完整退出。
func (OSRunner) Run(request Request) error {
	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
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
	return run(ctx, command)
}
