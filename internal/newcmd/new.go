package newcmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"goark.dev/cli/internal/scaffold"
)

// Run 解析脚手架参数并创建项目。
func Run(args []string, dir string, out io.Writer, errOut io.Writer) int {
	if len(args) == 0 {
		Help(errOut)
		return 2
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		Help(out)
		return 0
	}

	outputDir := "."
	if dir != "" {
		outputDir = dir
	}
	spec := scaffold.AppSpec{Dir: outputDir}
	projectType := "app"
	flags := flag.NewFlagSet("goark new", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&projectType, "type", projectType, "Project type")
	flags.StringVar(&spec.ModulePath, "module", "", "Go module path")
	flags.StringVar(&spec.Dir, "dir", spec.Dir, "Output directory")
	flags.BoolVar(&spec.Force, "force", false, "Overwrite existing files")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			Help(out)
			return 0
		}
		_, _ = fmt.Fprintf(errOut, "%v\n\n", err)
		Help(errOut)
		return 2
	}
	if flags.NArg() > 1 {
		_, _ = fmt.Fprintf(errOut, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		Help(errOut)
		return 2
	}
	if flags.NArg() == 0 {
		_, _ = fmt.Fprintln(errOut, "缺少项目名")
		Help(errOut)
		return 2
	}
	projectName := flags.Arg(0)
	if spec.ModulePath == "" {
		spec.ModulePath = projectName
	}
	spec.Name = projectName
	switch projectType {
	case "app":
		spec.Type = scaffold.ProjectTypeApp
	case "web":
		spec.Type = scaffold.ProjectTypeWeb
	default:
		_, _ = fmt.Fprintf(errOut, "不支持的项目类型 %q，仅支持 app 或 web\n\n", projectType)
		Help(errOut)
		return 2
	}
	files, err := scaffold.CreateApp(spec)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "%v\n\n", err)
		Help(errOut)
		return 2
	}
	_, _ = fmt.Fprintf(errOut, "created %s\n", spec.Dir)
	for _, file := range files {
		_, _ = fmt.Fprintf(errOut, "  %s\n", file.Path)
	}
	return 0
}

// Help 输出 new 命令帮助。
func Help(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark new [-type app|web] [-module <module-path>] [-dir <path>] <name>

Flags:
  -type string       Project type: app or web. Defaults to app.
  -module string     Go module path. Defaults to the project name.
  -dir path          Output directory. Defaults to the current directory.
  -force             Overwrite existing files.

Examples:
  goark new abc
  goark new -module github.com/abc/abc abc
  goark new -type web -module github.com/abc/abc -dir abc abc

`)
}
