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
	switch args[0] {
	case "help", "-h", "--help":
		c.printNewHelp(c.Out)
		return 0
	case "app":
		return c.runNewApp(args[1:])
	default:
		_, _ = fmt.Fprintf(c.Err, "未知骨架: %s\n\n", args[0])
		c.printNewHelp(c.Err)
		return 2
	}
}

func (c Command) runNewApp(args []string) int {
	spec := scaffold.AppSpec{}
	flags := flag.NewFlagSet("goark new app", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&spec.ModulePath, "module", "", "Go module path")
	flags.StringVar(&spec.Dir, "dir", ".", "Output directory")
	flags.StringVar(&spec.Name, "name", "", "Application name")
	flags.BoolVar(&spec.Web, "web", false, "Generate a Goark Boot Web application")
	flags.BoolVar(&spec.Force, "force", false, "Overwrite existing files")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.printNewAppHelp(c.Out)
			return 0
		}
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printNewAppHelp(c.Err)
		return 2
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(c.Err, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		c.printNewAppHelp(c.Err)
		return 2
	}
	files, err := scaffold.CreateApp(spec)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printNewAppHelp(c.Err)
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
  goark new <scaffold> [flags]

Available scaffolds:
  app               Create a Goark application skeleton.

`)
}

func (c Command) printNewAppHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark new app --module <module-path> --web [flags]

Flags:
  --module string    Required Go module path for the new application.
  --dir path         Output directory. Defaults to current directory.
  --name string      Application name. Defaults to the module path base name.
  --web              Generate a Goark Boot Web application.
  --force            Overwrite existing files.

Examples:
  goark new app --module example.com/admin --dir admin --web

`)
}
