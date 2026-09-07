package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/processrun"
	"goark.dev/cli/internal/runargs"
)

type workflowControl = buildplan.Control

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
	workingDir, err := runargs.EffectiveWorkingDir(c.Dir, goArguments)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, resolveErr := c.resolveProject(
		workingDir,
		nil,
		discoveryBuildFlags(goArguments),
		control.DryRun,
	)
	if resolveErr != nil {
		_, _ = fmt.Fprintln(c.Err, resolveErr)
		return projectResolutionExitCode(resolveErr)
	}
	plan, err := buildplan.Create(
		project.Build,
		command,
		control,
		goArguments,
		nil,
		nil,
		c.environment(),
	)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, resolveErr = c.resolveProject(
		workingDir,
		nil,
		discoveryBuildFlags(plan.GoArguments),
		control.DryRun,
	)
	if resolveErr != nil {
		_, _ = fmt.Fprintln(c.Err, resolveErr)
		return projectResolutionExitCode(resolveErr)
	}
	goArguments, err = applyDefaultBuildTarget(project, workingDir, command, plan.GoArguments)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	goCommand := composeEnhancedGoArguments(
		command,
		applyCommandOutput(command, goArguments, plan.Output),
	)
	return c.executeEnhancedLifecycle(command, project, plan, goCommand)
}

func (c Command) runApplication(args []string) int {
	if isHelpOnly(args) {
		c.printRunHelp(c.Out)
		return 0
	}
	plan, err := runargs.Parse(args)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	workingDir, err := runargs.EffectiveWorkingDir(c.Dir, plan.GoArguments)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, resolveErr := c.resolveProject(
		workingDir,
		nil,
		discoveryBuildFlags(plan.GoArguments),
		plan.Control.DryRun,
	)
	if resolveErr != nil {
		_, _ = fmt.Fprintln(c.Err, resolveErr)
		return projectResolutionExitCode(resolveErr)
	}
	if !plan.TargetExplicit {
		target, targetErr := project.ResolveRunTarget(workingDir)
		if targetErr != nil {
			_, _ = fmt.Fprintln(c.Err, targetErr)
			return 2
		}
		plan = plan.WithResolvedTarget(target)
	}
	commandPlan, err := buildplan.Create(
		project.Build,
		"run",
		plan.Control,
		plan.GoArguments,
		plan.PropertyArguments,
		plan.ApplicationArguments,
		c.environment(),
	)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, resolveErr = c.resolveProject(
		workingDir,
		nil,
		discoveryBuildFlags(commandPlan.GoArguments),
		plan.Control.DryRun,
	)
	if resolveErr != nil {
		_, _ = fmt.Fprintln(c.Err, resolveErr)
		return projectResolutionExitCode(resolveErr)
	}
	goArguments := composeEnhancedGoArguments("run", commandPlan.GoArguments)
	goArguments = append(goArguments, commandPlan.PropertyArguments...)
	goArguments = append(goArguments, commandPlan.ApplicationArguments...)
	return c.executeEnhancedLifecycle("run", project, commandPlan, goArguments)
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
	workingDir, err := runargs.EffectiveWorkingDir(c.Dir, directoryFlags)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, err := c.resolveProject(workingDir, patterns, buildFlags, control.DryRun)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return projectResolutionExitCode(err)
	}
	plan, err := buildplan.Create(
		project.Build,
		"generate",
		control,
		buildFlags,
		nil,
		nil,
		c.environment(),
	)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, err = c.resolveProject(
		workingDir,
		patterns,
		discoveryBuildFlags(plan.GoArguments),
		control.DryRun,
	)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return projectResolutionExitCode(err)
	}
	return c.executeGenerateLifecycle(project, plan)
}

func parseProjectGenerationArguments(
	args []string,
) ([]string, []string, []string, workflowControl, error) {
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
					return nil, nil, nil, workflowControl{}, fmt.Errorf("go 参数 -C 缺少目录")
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
					return nil, nil, nil, workflowControl{}, fmt.Errorf("go 构建参数 %s 缺少值", arg)
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

func (c Command) resolveProject(
	dir string,
	patterns []string,
	buildFlags []string,
	static bool,
) (goarkProject, error) {
	return projectResolver{
		Context:    c.Context,
		Dir:        runargs.BaseDir(dir),
		Env:        append([]string(nil), c.Env...),
		Runner:     c.Runner,
		Err:        c.Err,
		Patterns:   append([]string(nil), patterns...),
		BuildFlags: append([]string(nil), buildFlags...),
		Static:     static,
	}.Resolve()
}

func projectResolutionExitCode(err error) int {
	if errors.Is(err, context.Canceled) {
		return 130
	}
	if code, ok := processrun.ExitCode(err); ok {
		return code
	}
	return 2
}

func applyCommandOutput(command string, arguments []string, output string) []string {
	result := append([]string(nil), arguments...)
	if command != "build" || output == "" || hasGoOutputFlag(result) {
		return result
	}
	return append([]string{"-o", output}, result...)
}

func applyDefaultBuildTarget(
	project goarkProject,
	workingDir string,
	command string,
	arguments []string,
) ([]string, error) {
	result := append([]string(nil), arguments...)
	if command != "build" || project.Build.Project.Main == "" || hasBuildTarget(result) {
		return result, nil
	}
	target, err := project.ResolveRunTarget(workingDir)
	if err != nil {
		return nil, err
	}
	return append(result, target), nil
}

func hasBuildTarget(arguments []string) bool {
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if strings.HasPrefix(argument, "-") {
			if runargs.BuildFlagConsumesValue(argument) && index+1 < len(arguments) {
				index++
			}
			continue
		}
		return true
	}
	return false
}

func hasGoOutputFlag(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "-o" || strings.HasPrefix(argument, "-o=") {
			return true
		}
	}
	return false
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
