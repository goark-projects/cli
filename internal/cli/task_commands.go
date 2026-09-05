package cli

import (
	"fmt"
	"io"
	"strings"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/taskgraph"
	"goark.dev/cli/internal/taskview"
)

func (c Command) runTasks(args []string) int {
	if isHelpOnly(args) {
		c.printTasksHelp(c.Out)
		return 0
	}
	jsonOutput := false
	for _, argument := range args {
		if argument != "--json" || jsonOutput {
			_, _ = fmt.Fprintf(c.Err, "goark tasks 不接受参数: %s\n", strings.Join(args, " "))
			return 2
		}
		jsonOutput = true
	}
	project, err := c.resolveProjectMetadata(c.Dir)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	if err := validateProjectTaskGraph(project); err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	if err := taskview.WriteList(c.Out, taskview.Snapshot(project.Build.Tasks), jsonOutput); err != nil {
		_, _ = fmt.Fprintf(c.Err, "输出任务列表失败: %v\n", err)
		return 1
	}
	return 0
}

func (c Command) runGraph(args []string) int {
	if isHelpOnly(args) {
		c.printGraphHelp(c.Out)
		return 0
	}
	format := taskview.FormatText
	if len(args) > 1 {
		return c.graphUsageError(args)
	}
	if len(args) == 1 {
		if !strings.HasPrefix(args[0], "--format=") {
			return c.graphUsageError(args)
		}
		format = taskview.Format(strings.TrimPrefix(args[0], "--format="))
		if format != taskview.FormatText && format != taskview.FormatJSON && format != taskview.FormatDOT {
			return c.graphUsageError(args)
		}
	}
	project, err := c.resolveProjectMetadata(c.Dir)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	if err := validateProjectTaskGraph(project); err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	if err := taskview.WriteGraph(c.Out, taskview.Snapshot(project.Build.Tasks), format); err != nil {
		_, _ = fmt.Fprintf(c.Err, "输出任务图失败: %v\n", err)
		return 1
	}
	return 0
}

func (c Command) graphUsageError(args []string) int {
	_, _ = fmt.Fprintf(c.Err, "goark graph 仅支持 --format=text、json、dot: %s\n", strings.Join(args, " "))
	return 2
}

func (c Command) runTask(args []string) int {
	if isHelpOnly(args) {
		c.printTaskHelp(c.Out)
		return 0
	}
	remaining, control, err := buildplan.ParseControlArguments(args)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	if len(remaining) != 1 || strings.HasPrefix(remaining[0], "-") {
		_, _ = fmt.Fprintf(c.Err, "Usage: goark task <name> [--goark-*]\n")
		return 2
	}
	project, err := c.resolveProjectMetadata(c.Dir)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	name := remaining[0]
	if _, ok := project.Build.Tasks[name]; !ok {
		_, _ = fmt.Fprintf(c.Err, "任务 %q 不存在\n", name)
		return 2
	}
	plan, err := buildplan.Create(project.Build, "task", control, nil, nil, nil, c.environment())
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	lifecycle, err := c.prepareLifecycleTargets(project, plan, []string{name})
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	return lifecycle.withProjectLock(func() int {
		return lifecycle.executeTasks(c.Context, []string{name})
	})
}

func validateProjectTaskGraph(project goarkProject) error {
	if _, err := taskgraph.New(project.Build.Tasks); err != nil {
		return err
	}
	return validateTaskPaths(project)
}

func (c Command) resolveProjectMetadata(dir string) (goarkProject, error) {
	return projectResolver{
		Context: c.Context, Dir: effectiveBaseDir(dir), Env: append([]string(nil), c.Env...), Runner: c.Runner,
		Err: c.Err, Static: true, MetadataOnly: true,
	}.Resolve()
}

func (c Command) printTasksHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark tasks [--json]\n")
}

func (c Command) printTaskHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark task <name> [--goark-profile=<name>] [--goark-dry-run]\n")
}

func (c Command) printGraphHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark graph [--format=text|json|dot]\n")
}
