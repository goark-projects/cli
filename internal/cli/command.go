package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// Version 是开发期版本号，正式发布时由构建流程通过 ldflags 覆盖。
var Version = "0.1.0-dev"

// Command 封装命令执行所需的输入参数与输出边界。
type Command struct {
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Dir    string
	Env    []string
	Runner ProcessRunner
}

// Main 执行 CLI 主流程，并返回进程退出码。
func Main(args []string, stdout io.Writer, stderr io.Writer) int {
	cmd := Command{
		Out: stdout,
		Err: stderr,
	}
	return cmd.Run(args)
}

// Run 根据首个参数分发命令。
func (c Command) Run(args []string) int {
	c = c.withDefaults()
	if len(args) == 0 {
		c.printHelp(c.Out)
		return 0
	}

	switch args[0] {
	case "-h", "--help":
		c.printHelp(c.Out)
		return 0
	case "help":
		return c.runHelp(args[1:])
	case "-v", "--version":
		_, _ = fmt.Fprintf(c.Out, "goark %s\n", Version)
		return 0
	case "version":
		return c.runVersion(args[1:])
	case "go":
		return c.runGo(args[1:])
	case "new":
		return c.runNew(args[1:])
	case "run":
		return c.runApplication(args[1:])
	case "build", "test", "install", "vet", "list", "fix":
		return c.runEnhancedGo(args[0], args[1:])
	case "generate":
		return c.runProjectGenerate(args[1:])
	case "info":
		return c.runInfo(args[1:])
	case "codegen":
		return c.runCodegen(args[1:])
	case "completion":
		return c.runCompletion(args[1:])
	default:
		_, _ = fmt.Fprintf(c.Err, "未知命令: %s\n\n", args[0])
		c.printHelp(c.Err)
		return 2
	}
}

func (c Command) runVersion(args []string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintf(c.Out, "goark %s\n", Version)
		return 0
	}
	if isHelpOnly(args) {
		c.printVersionHelp(c.Out)
		return 0
	}
	_, _ = fmt.Fprintf(c.Err, "goark version 不接受参数: %s\n", strings.Join(args, " "))
	return 2
}

func (c Command) withDefaults() Command {
	if c.Out == nil {
		c.Out = io.Discard
	}
	if c.Err == nil {
		c.Err = io.Discard
	}
	if c.Runner == nil {
		c.Runner = osProcessRunner{}
	}
	return c
}

func (c Command) runGo(args []string) int {
	err := c.Runner.Run(ProcessRequest{
		Name: "go",
		Args: append([]string(nil), args...),
		Dir:  c.Dir,
		Env:  append([]string(nil), c.Env...),
		In:   c.In,
		Out:  c.Out,
		Err:  c.Err,
	})
	if err == nil {
		return 0
	}
	var exitError interface{ ExitCode() int }
	if errors.As(err, &exitError) {
		if code := exitError.ExitCode(); code >= 0 {
			return code
		}
	}
	if code, ok := signaledProcessExitCode(err); ok {
		return code
	}
	_, _ = fmt.Fprintf(c.Err, "启动 go 失败: %v\n", err)
	return 1
}

func (c Command) printHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `goark is the command-line tool for the Goark ecosystem.

Usage:
  goark <command> [arguments]

Available commands:
  help              Show command help.
  version           Print the CLI version.
  new               Create project skeletons.
  run               Generate code and run a Goark application.
  build             Generate code and build packages.
  test              Generate code and test packages.
  install           Generate code and install packages.
  vet               Generate code and report package problems.
  list              Generate code and list packages.
  fix               Generate code and update packages.
  generate          Generate all Goark compile-time code.
  codegen           Run a low-level Goark code generator.
  info              Show the current Goark project diagnostics.
  go                Run the official Go command without Goark extensions.
  completion        Generate shell completion scripts.

Available scaffolds:
  app               Create a Goark application skeleton.

`)
}
