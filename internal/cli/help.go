package cli

import (
	"fmt"
	"io"
	"strings"
)

var enhancedGoCommands = map[string]struct{}{
	"build":   {},
	"test":    {},
	"install": {},
	"vet":     {},
	"list":    {},
	"fix":     {},
}

func (c Command) runHelp(args []string) int {
	if len(args) == 0 {
		c.printHelp(c.Out)
		return 0
	}
	if len(args) > 2 {
		_, _ = fmt.Fprintf(c.Err, "help 参数过多: %s\n", strings.Join(args, " "))
		return 2
	}
	command := args[0]
	switch command {
	case "help":
		if len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printHelpHelp(c.Out)
	case "version":
		if len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printVersionHelp(c.Out)
	case "new":
		if len(args) == 2 && args[1] == "app" {
			c.printNewAppHelp(c.Out)
		} else if len(args) == 1 {
			c.printNewHelp(c.Out)
		} else {
			return c.unknownHelpCommand(args)
		}
	case "run":
		if len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printRunHelp(c.Out)
	case "generate":
		if len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printProjectGenerateHelp(c.Out)
	case "info":
		if len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printInfoHelp(c.Out)
	case "go":
		if len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printGoHelp(c.Out)
	case "completion":
		if len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printCompletionHelp(c.Out)
	case "codegen":
		if len(args) == 1 {
			c.printCodegenHelp(c.Out)
			break
		}
		switch args[1] {
		case "configuration":
			c.printCodegenConfigurationHelp(c.Out)
		case "registry":
			c.printCodegenRegistryHelp(c.Out)
		case "annotations":
			c.printCodegenAnnotationsHelp(c.Out)
		default:
			return c.unknownHelpCommand(args)
		}
	default:
		if _, ok := enhancedGoCommands[command]; !ok || len(args) != 1 {
			return c.unknownHelpCommand(args)
		}
		c.printEnhancedGoHelp(c.Out, command)
	}
	return 0
}

func (c Command) unknownHelpCommand(args []string) int {
	_, _ = fmt.Fprintf(c.Err, "未知帮助主题: %s\n", strings.Join(args, " "))
	return 2
}

func (c Command) printHelpHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, "Usage:\n  goark help [command]\n")
}

func (c Command) printVersionHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, "Usage:\n  goark version\n")
}

func (c Command) printGoHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark go <go-arguments>

Runs the official Go command exactly as provided. Goark code generation is not executed.

Examples:
  goark go version
  goark go generate ./...
  goark go build ./...

`)
}

func (c Command) printEnhancedGoHelp(w io.Writer, command string) {
	_, _ = fmt.Fprintf(w, `Usage:
  goark %s [go-arguments]

Generates Goark compile-time code, then runs "go %s" with the original Go arguments.

Goark flags:
  --goark-no-generate     Skip compile-time generation.
  --goark-generate-only   Generate code without running Go.
  --goark-dry-run         Print the generation and Go command plan.

All other arguments are passed to the official Go command.

`, command, command)
}

func (c Command) printProjectGenerateHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark generate [package-patterns] [--goark-dry-run]

Generates all enabled Goark compile-time code for local package patterns.
Patterns default to ./....

Package loading flags:
  -C <directory>   Run from another directory.
  -tags <tags>     Select build-tagged source files.
  -mod <mode>      Set module download mode.
  -modfile <file>  Use an alternate go.mod file.
  -overlay <file>  Read a JSON overlay configuration.

`)
}

func (c Command) printInfoHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark info [--json]

Shows Goark CLI, Go toolchain, project and generation diagnostics.

`)
}
