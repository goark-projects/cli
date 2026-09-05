package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/processrun"
	"goark.dev/cli/internal/projectlock"
	"goark.dev/cli/internal/taskexec"
)

func (c Command) executeEnhancedLifecycle(command string, project goarkProject, plan buildplan.Plan, goArguments []string) int {
	lifecycle, err := c.prepareLifecycle(project, plan)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	return lifecycle.withProjectLock(func() int {
		return lifecycle.executeGoCommand(command, goArguments)
	})
}

func (c Command) executeGenerateLifecycle(project goarkProject, plan buildplan.Plan) int {
	lifecycle, err := c.prepareLifecycle(project, plan)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	return lifecycle.withProjectLock(func() int {
		return lifecycle.generate()
	})
}

func (l *preparedLifecycle) executeGoCommand(command string, goArguments []string) int {
	configuration := l.project.Build.Commands[command]
	code := 0
	if command == "fix" {
		code = l.executeTasks(l.command.Context, configuration.Before)
		if code == 0 {
			code = l.runGo(goArguments)
		}
		if code == 0 {
			code = l.generate()
		}
	} else {
		code = l.generate()
		if code == 0 {
			code = l.executeTasks(l.command.Context, configuration.Before)
		}
		if code == 0 {
			code = l.runGo(goArguments)
		}
	}
	if code == 0 {
		code = l.executeTasks(l.command.Context, configuration.After)
	}
	return l.finish(code, configuration.Finally)
}

func (l *preparedLifecycle) generate() int {
	configuration := l.project.Build.Commands["generate"]
	code := l.executeTasks(l.command.Context, configuration.Before)
	if code == 0 {
		code = l.command.generateAndReport(l.project, l.plan.Control.DryRun)
	}
	if code == 0 {
		code = l.executeTasks(l.command.Context, configuration.After)
	}
	return l.finish(code, configuration.Finally)
}

func (l *preparedLifecycle) executeTasks(ctx context.Context, names []string) int {
	if len(names) == 0 {
		return 0
	}
	executor := taskexec.Executor{
		Graph: l.graph, MaxParallel: l.project.Build.Execution.MaxParallel,
		FailFast: l.project.Build.Execution.FailFast, Runner: l.taskRunner,
	}
	if err := executor.Execute(ctx, names); err != nil {
		_, _ = fmt.Fprintln(l.command.Err, err)
		return executionErrorCode(err)
	}
	return 0
}

func (l *preparedLifecycle) executeFinally(names []string) error {
	if len(names) == 0 {
		return nil
	}
	order, err := l.graph.Order(names)
	if err != nil {
		return err
	}
	timeout := l.project.Build.Execution.DefaultTimeout.Duration
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var first error
	for index := len(order) - 1; index >= 0; index-- {
		name := order[index]
		task, _ := l.graph.Task(name)
		if err := l.taskRunner.Run(ctx, name, task); err != nil {
			if first == nil {
				first = err
			}
			_, _ = fmt.Fprintf(l.command.Err, "finally 任务 %q 失败: %v\n", name, err)
		}
	}
	return first
}

func (l *preparedLifecycle) finish(code int, finally []string) int {
	err := l.executeFinally(finally)
	if code == 0 && err != nil {
		return executionErrorCode(err)
	}
	return code
}

func (l *preparedLifecycle) runGo(arguments []string) int {
	if l.plan.Control.DryRun {
		_, _ = fmt.Fprintf(l.command.Err, "would run: go %s\n", joinArguments(arguments, l.plan.Environment))
		return 0
	}
	configured := l.command
	configured.Env = l.plan.EnvironmentList()
	return configured.runGo(arguments)
}

func (l *preparedLifecycle) withProjectLock(run func() int) int {
	if l.plan.Control.DryRun {
		return run()
	}
	lock, err := projectlock.Acquire(l.command.Context, l.project.Root)
	if err != nil {
		_, _ = fmt.Fprintln(l.command.Err, err)
		return executionErrorCode(err)
	}
	code := run()
	if err := lock.Release(); err != nil {
		_, _ = fmt.Fprintln(l.command.Err, err)
		if code == 0 {
			return 1
		}
	}
	return code
}

func executionErrorCode(err error) int {
	if code, ok := processrun.ExitCode(err); ok {
		return code
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 1
}

func joinArguments(arguments []string, environment map[string]string) string {
	arguments = buildplan.RedactArguments(arguments, environment)
	for index, argument := range arguments {
		if argument == "" || strings.ContainsAny(argument, " \t\r\n\"'") {
			arguments[index] = strconv.Quote(argument)
		}
	}
	return strings.Join(arguments, " ")
}
