package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"goark.dev/cli/internal/scaffold"
)

func (c Command) runNew(args []string) int {
	if len(args) == 0 {
		c.printNewHelp(c.Err)
		return 2
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		c.printNewHelp(c.Out)
		return 0
	}

	outputDir := "."
	if c.Dir != "" {
		outputDir = c.Dir
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
			c.printNewHelp(c.Out)
			return 0
		}
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printNewHelp(c.Err)
		return 2
	}
	if flags.NArg() > 1 {
		_, _ = fmt.Fprintf(c.Err, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		c.printNewHelp(c.Err)
		return 2
	}
	if flags.NArg() == 0 {
		_, _ = fmt.Fprintln(c.Err, "缺少项目名")
		c.printNewHelp(c.Err)
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
		_, _ = fmt.Fprintf(c.Err, "不支持的项目类型 %q，仅支持 app 或 web\n\n", projectType)
		c.printNewHelp(c.Err)
		return 2
	}
	files, err := scaffold.CreateApp(spec)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printNewHelp(c.Err)
		return 2
	}
	_, _ = fmt.Fprintf(c.Err, "created %s\n", spec.Dir)
	for _, file := range files {
		_, _ = fmt.Fprintf(c.Err, "  %s\n", file.Path)
	}
	return 0
}

func (c Command) printNewHelp(w io.Writer) {
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
