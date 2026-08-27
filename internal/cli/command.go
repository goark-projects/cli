package cli

import (
	"fmt"
	"io"
)

// Version 是开发期版本号，正式发布时由构建流程覆盖。
const Version = "0.1.0-dev"

// Command 封装命令执行所需的输入参数与输出边界。
type Command struct {
	Out io.Writer
	Err io.Writer
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
	if len(args) == 0 {
		c.printHelp(c.Out)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		c.printHelp(c.Out)
		return 0
	case "version", "-v", "--version":
		_, _ = fmt.Fprintf(c.Out, "goark %s\n", Version)
		return 0
	case "generate", "gen":
		return c.runGenerate(args[1:])
	default:
		_, _ = fmt.Fprintf(c.Err, "未知命令: %s\n\n", args[0])
		c.printHelp(c.Err)
		return 2
	}
}

func (c Command) printHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `goark is the command-line tool for the Goark ecosystem.

Usage:
  goark <command> [arguments]

Available commands:
  help              Show command help.
  version           Print the CLI version.
  generate, gen     Run code generators.

Available generators:
  configuration     Generate a goark.Configuration source file.
  registry          Generate a Configuration registry source file.
  annotations       Scan //goark annotations and generate registration code.

`)
}
