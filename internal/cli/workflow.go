package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type workflowControl struct {
	SkipGenerate bool
	GenerateOnly bool
	DryRun       bool
}

func (c Command) runEnhancedGo(command string, args []string) int {
	if isHelpOnly(args) {
		c.printEnhancedGoHelp(c.Out, command)
		return 0
	}
	goArguments, control, err := parseWorkflowArguments(args)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	workingDir, err := effectiveGoWorkingDir(c.Dir, goArguments)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	remoteInstall := command == "install" && containsVersionedPackage(goArguments)
	if remoteInstall && control.GenerateOnly {
		_, _ = fmt.Fprintln(c.Err, "远程版本化 package 不支持 --goark-generate-only")
		return 2
	}
	if !control.SkipGenerate && !remoteInstall {
		project, resolveErr := c.resolveProject(workingDir, nil, discoveryBuildFlags(goArguments))
		if resolveErr != nil {
			_, _ = fmt.Fprintln(c.Err, resolveErr)
			return 2
		}
		if code := c.generateAndReport(project, control.DryRun); code != 0 {
			return code
		}
	}
	goCommand := composeEnhancedGoArguments(command, goArguments)
	if control.DryRun {
		_, _ = fmt.Fprintf(c.Err, "would run: go %s\n", strings.Join(goCommand, " "))
		return 0
	}
	if control.GenerateOnly {
		return 0
	}
	return c.runGo(goCommand)
}

func containsVersionedPackage(args []string) bool {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if strings.HasPrefix(arg, "-") {
			if goBuildFlagConsumesValue(arg) && index+1 < len(args) {
				index++
			}
			continue
		}
		if strings.Contains(arg, "@") {
			return true
		}
	}
	return false
}

func (c Command) runApplication(args []string) int {
	if isHelpOnly(args) {
		c.printRunHelp(c.Out)
		return 0
	}
	plan, err := parseRunArguments(args)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	workingDir, err := effectiveGoWorkingDir(c.Dir, plan.GoArguments)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	remoteTarget := plan.TargetExplicit && strings.Contains(plan.Target, "@")
	if remoteTarget && plan.GenerateOnly {
		_, _ = fmt.Fprintln(c.Err, "远程版本化 package 不支持 --goark-generate-only")
		return 2
	}
	needsProject := !plan.SkipGenerate || !plan.TargetExplicit
	if needsProject && !remoteTarget {
		project, resolveErr := c.resolveProject(workingDir, nil, discoveryBuildFlags(plan.GoArguments))
		if resolveErr != nil {
			_, _ = fmt.Fprintln(c.Err, resolveErr)
			return 2
		}
		if !plan.TargetExplicit {
			target, targetErr := project.ResolveRunTarget(workingDir)
			if targetErr != nil {
				_, _ = fmt.Fprintln(c.Err, targetErr)
				return 2
			}
			plan = plan.WithResolvedTarget(target)
		}
		if !plan.SkipGenerate {
			if code := c.generateAndReport(project, plan.DryRun); code != 0 {
				return code
			}
		}
	}
	goArguments := composeEnhancedGoArguments("run", plan.GoArguments)
	goArguments = append(goArguments, plan.PropertyArguments...)
	goArguments = append(goArguments, plan.ApplicationArguments...)
	if plan.DryRun {
		_, _ = fmt.Fprintf(c.Err, "would run: go %s\n", strings.Join(goArguments, " "))
		return 0
	}
	if plan.GenerateOnly {
		return 0
	}
	return c.runGo(goArguments)
}

func (c Command) runProjectGenerate(args []string) int {
	if isHelpOnly(args) {
		c.printProjectGenerateHelp(c.Out)
		return 0
	}
	patterns, buildFlags, directoryFlags, control, err := parseProjectGenerationArguments(args)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	if control.SkipGenerate || control.GenerateOnly {
		_, _ = fmt.Fprintln(c.Err, "goark generate 不支持 --goark-no-generate 或 --goark-generate-only")
		return 2
	}
	workingDir, err := effectiveGoWorkingDir(c.Dir, directoryFlags)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, err := c.resolveProject(workingDir, patterns, buildFlags)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	return c.generateAndReport(project, control.DryRun)
}

func parseProjectGenerationArguments(args []string) ([]string, []string, []string, workflowControl, error) {
	remaining, control, err := parseWorkflowArguments(args)
	if err != nil {
		return nil, nil, nil, workflowControl{}, err
	}
	patterns := make([]string, 0)
	buildFlags := make([]string, 0)
	directoryFlags := make([]string, 0, 2)
	for index := 0; index < len(remaining); index++ {
		arg := remaining[index]
		name := arg
		if separator := strings.IndexByte(name, '='); separator >= 0 {
			name = name[:separator]
		}
		switch {
		case name == "-C":
			directoryFlags = append(directoryFlags, arg)
			if !strings.Contains(arg, "=") {
				if index+1 >= len(remaining) {
					return nil, nil, nil, workflowControl{}, fmt.Errorf("Go 参数 -C 缺少目录")
				}
				index++
				directoryFlags = append(directoryFlags, remaining[index])
			}
		case hasFlag(discoveryBooleanFlags, name):
			buildFlags = append(buildFlags, arg)
		case hasFlag(discoveryValueFlags, name):
			buildFlags = append(buildFlags, arg)
			if !strings.Contains(arg, "=") {
				if index+1 >= len(remaining) {
					return nil, nil, nil, workflowControl{}, fmt.Errorf("Go 构建参数 %s 缺少值", arg)
				}
				index++
				buildFlags = append(buildFlags, remaining[index])
			}
		case strings.HasPrefix(arg, "-"):
			return nil, nil, nil, workflowControl{}, fmt.Errorf("未知 goark generate 参数: %s", arg)
		default:
			if err := validateLocalProjectPattern(arg); err != nil {
				return nil, nil, nil, workflowControl{}, fmt.Errorf("生成范围无效: %w", err)
			}
			patterns = append(patterns, arg)
		}
	}
	return patterns, buildFlags, directoryFlags, control, nil
}

func (c Command) runInfo(args []string) int {
	jsonOutput := false
	if len(args) > 0 {
		if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
			c.printInfoHelp(c.Out)
			return 0
		}
		if len(args) == 1 && args[0] == "--json" {
			jsonOutput = true
		} else {
			_, _ = fmt.Fprintf(c.Err, "goark info 不接受参数: %s\n", strings.Join(args, " "))
			return 2
		}
	}
	project, err := c.resolveProject(c.Dir, nil, nil)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	results, err := generateProject(project, true)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 1
	}
	mainTarget, mainErr := project.ResolveRunTarget(effectiveBaseDir(c.Dir))
	if mainErr != nil {
		mainTarget = "unresolved (" + mainErr.Error() + ")"
	}
	goVersion := c.captureGoVersion()
	if jsonOutput {
		info := projectInfo{
			CLIVersion:         Version,
			GoToolchain:        goVersion,
			Module:             project.ModulePath,
			Root:               project.Root,
			Main:               mainTarget,
			Generators:         []string{"annotations"},
			GenerationPatterns: []string{"./..."},
			GeneratedPackages:  len(results),
		}
		encoder := json.NewEncoder(c.Out)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(info); err != nil {
			_, _ = fmt.Fprintf(c.Err, "输出 info JSON 失败: %v\n", err)
			return 1
		}
		return 0
	}
	_, _ = fmt.Fprintf(c.Out, "Goark CLI: %s\n", Version)
	_, _ = fmt.Fprintf(c.Out, "Go toolchain: %s\n", goVersion)
	_, _ = fmt.Fprintf(c.Out, "Module: %s\n", project.ModulePath)
	_, _ = fmt.Fprintf(c.Out, "Root: %s\n", project.Root)
	_, _ = fmt.Fprintf(c.Out, "Main: %s\n", mainTarget)
	_, _ = fmt.Fprintln(c.Out, "Generators: annotations")
	_, _ = fmt.Fprintln(c.Out, "Generation patterns: ./...")
	_, _ = fmt.Fprintf(c.Out, "Generated packages: %d\n", len(results))
	return 0
}

type projectInfo struct {
	CLIVersion         string   `json:"cliVersion"`
	GoToolchain        string   `json:"goToolchain"`
	Module             string   `json:"module"`
	Root               string   `json:"root"`
	Main               string   `json:"main"`
	Generators         []string `json:"generators"`
	GenerationPatterns []string `json:"generationPatterns"`
	GeneratedPackages  int      `json:"generatedPackages"`
}

func (c Command) resolveProject(dir string, patterns []string, buildFlags []string) (goarkProject, error) {
	return projectResolver{
		Dir:        effectiveBaseDir(dir),
		Env:        append([]string(nil), c.Env...),
		Runner:     c.Runner,
		Err:        c.Err,
		Patterns:   append([]string(nil), patterns...),
		BuildFlags: append([]string(nil), buildFlags...),
	}.Resolve()
}

func discoveryBuildFlags(args []string) []string {
	flags := make([]string, 0)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		name := arg
		if separator := strings.IndexByte(name, '='); separator >= 0 {
			name = name[:separator]
		}
		if hasFlag(discoveryBooleanFlags, name) {
			flags = append(flags, arg)
			continue
		}
		if !hasFlag(discoveryValueFlags, name) {
			continue
		}
		flags = append(flags, arg)
		if !strings.Contains(arg, "=") && index+1 < len(args) {
			index++
			flags = append(flags, args[index])
		}
	}
	return flags
}

var discoveryValueFlags = map[string]struct{}{
	"-mod": {}, "-modfile": {}, "-overlay": {}, "-tags": {},
}

var discoveryBooleanFlags = map[string]struct{}{
	"-asan": {}, "-msan": {}, "-race": {}, "-trimpath": {},
}

func hasFlag(flags map[string]struct{}, name string) bool {
	_, ok := flags[name]
	return ok
}

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
		Name: "go",
		Args: []string{"version"},
		Dir:  c.Dir,
		Env:  append([]string(nil), c.Env...),
		Out:  &output,
		Err:  io.Discard,
	})
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(output.String())
}

func parseWorkflowArguments(args []string) ([]string, workflowControl, error) {
	control := workflowControl{}
	goArguments := make([]string, 0, len(args))
	passthrough := false
	for _, arg := range args {
		if passthrough {
			goArguments = append(goArguments, arg)
			continue
		}
		if arg == "--" || arg == "-args" {
			passthrough = true
			goArguments = append(goArguments, arg)
			continue
		}
		switch arg {
		case "--goark-no-generate":
			control.SkipGenerate = true
		case "--goark-generate-only":
			control.GenerateOnly = true
		case "--goark-dry-run":
			control.DryRun = true
		default:
			if strings.HasPrefix(arg, "--goark-") {
				return nil, workflowControl{}, fmt.Errorf("未知 Goark 参数: %s", arg)
			}
			goArguments = append(goArguments, arg)
		}
	}
	if control.SkipGenerate && control.GenerateOnly {
		return nil, workflowControl{}, fmt.Errorf("--goark-no-generate 不能与 --goark-generate-only 同时使用")
	}
	return goArguments, control, nil
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
  --goark-no-generate     Skip compile-time generation.
  --goark-generate-only   Generate code without running the application.
  --goark-dry-run         Print the generation and Go command plan.

`)
}
