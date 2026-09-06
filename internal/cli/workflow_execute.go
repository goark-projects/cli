package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"goark.dev/cli/internal/buildplan"
)

func (c Command) generateAndReport(project goarkProject, dryRun bool) int {
	results, err := generateProject(project, dryRun)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 1
	}
	for _, result := range results {
		switch {
		case dryRun && result.Removed:
			_, _ = fmt.Fprintf(c.Err, "would remove %s\n", result.Output)
		case dryRun:
			_, _ = fmt.Fprintf(c.Err, "would generate %s\n", result.Output)
		case result.Removed:
			_, _ = fmt.Fprintf(c.Err, "removed %s\n", result.Output)
		case result.Changed:
			_, _ = fmt.Fprintf(c.Err, "generated %s\n", result.Output)
		}
	}
	return 0
}

func (c Command) captureGoVersion() string {
	var output bytes.Buffer
	err := c.Runner.Run(ProcessRequest{
		Context: c.Context,
		Name:    "go",
		Args:    []string{"version"},
		Dir:     c.Dir,
		Env:     append([]string(nil), c.Env...),
		Out:     &output,
		Err:     io.Discard,
	})
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(output.String())
}

func parseWorkflowArguments(args []string) ([]string, workflowControl, error) {
	return buildplan.ParseControlArguments(args)
}

func effectiveGoWorkingDir(base string, args []string) (string, error) {
	workingDir := effectiveBaseDir(base)
	directory := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		var value string
		switch {
		case arg == "-C":
			if index+1 >= len(args) {
				return "", fmt.Errorf("Go 参数 -C 缺少目录")
			}
			value = args[index+1]
		case strings.HasPrefix(arg, "-C="):
			value = strings.TrimPrefix(arg, "-C=")
		}
		if value == "" {
			continue
		}
		directory = value
	}
	if directory != "" {
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(workingDir, directory)
		}
		return filepath.Clean(directory), nil
	}
	return workingDir, nil
}

func effectiveBaseDir(dir string) string {
	if dir != "" {
		return filepath.Clean(dir)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return workingDir
}

func isHelpOnly(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "--help")
}

func composeEnhancedGoArguments(command string, args []string) []string {
	global := make([]string, 0, 2)
	commandArgs := make([]string, 0, len(args))
	passthrough := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if passthrough {
			commandArgs = append(commandArgs, arg)
			continue
		}
		if arg == "--" || arg == "-args" {
			passthrough = true
			commandArgs = append(commandArgs, arg)
			continue
		}
		switch {
		case arg == "-C" && index+1 < len(args):
			global = append(global, arg, args[index+1])
			index++
		case strings.HasPrefix(arg, "-C="):
			global = append(global, arg)
		default:
			commandArgs = append(commandArgs, arg)
		}
	}
	result := make([]string, 0, len(global)+1+len(commandArgs))
	result = append(result, global...)
	result = append(result, command)
	return append(result, commandArgs...)
}

func (c Command) printRunHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark run [go-build-flags] [package-or-go-files] [properties] [-- application-arguments]

Goark flags:
  --goark-profile=<name>  Select a declared build Profile.
  --goark-dry-run         Print the complete plan without side effects or processes.
  --goark-offline         Forbid network access and automatic tool restoration.
  --goark-locked          Require the existing lock file without updating it.
  --goark-env=KEY=VALUE   Override one environment variable; repeat as needed.

`)
}
