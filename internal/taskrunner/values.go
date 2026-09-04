package taskrunner

import (
	"fmt"
	"path/filepath"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/expand"
	"goark.dev/cli/internal/projectfs"
	"goark.dev/cli/internal/taskcache"
)

func (r *Runner) taskValues(task buildspec.Task) (map[string]string, expand.Values, error) {
	environment := cloneMap(r.options.Environment)
	tools := make(map[string]string, len(r.options.Tools))
	for name, tool := range r.options.Tools {
		tools[name] = tool.Path
	}
	values := expand.Values{
		ProjectRoot: r.options.Root, ProjectName: r.options.ProjectName, ProjectModule: r.options.ProjectModule,
		Profile: r.options.Profile, CommandOutput: r.options.CommandOutput, Tools: tools, Environment: environment,
	}
	for name, raw := range task.Environment {
		value, err := expand.String(raw, values)
		if err != nil {
			return nil, expand.Values{}, fmt.Errorf("替换任务环境变量 %s 失败: %w", name, err)
		}
		environment[name] = value
	}
	for name, value := range r.options.OverrideEnvironment {
		environment[name] = value
	}
	values.Environment = environment
	return environment, values, nil
}

func expandTask(task buildspec.Task, values expand.Values) (buildspec.Task, error) {
	var err error
	if task.Args, err = expand.Strings(task.Args, values); err != nil {
		return buildspec.Task{}, err
	}
	if task.Inputs, err = expand.Strings(task.Inputs, values); err != nil {
		return buildspec.Task{}, fmt.Errorf("替换 inputs 失败: %w", err)
	}
	if task.Outputs, err = expand.Strings(task.Outputs, values); err != nil {
		return buildspec.Task{}, fmt.Errorf("替换 outputs 失败: %w", err)
	}
	if task.WorkingDirectory != "" {
		task.WorkingDirectory, err = expand.String(task.WorkingDirectory, values)
		if err != nil {
			return buildspec.Task{}, fmt.Errorf("替换 working-directory 失败: %w", err)
		}
	}
	return task, nil
}

func (r *Runner) workingDirectory(value string) (string, error) {
	if value == "" {
		return filepath.Clean(r.options.Root), nil
	}
	return projectfs.New(r.options.Root).Resolve(value, projectfs.MustExist)
}

func (r *Runner) cacheContext(name string, task buildspec.Task, environment map[string]string) taskcache.Context {
	upstream := make(map[string]string, len(task.DependsOn))
	r.mu.Lock()
	for _, dependency := range task.DependsOn {
		upstream[dependency] = r.upstream[dependency]
	}
	r.mu.Unlock()
	context := taskcache.Context{
		Root: r.options.Root, TaskName: name, Task: task, GoVersion: r.options.GoVersion,
		GOOS: r.options.GOOS, GOARCH: r.options.GOARCH, BuildTags: r.options.BuildTags,
		Profile: r.options.Profile, Environment: environment, Upstream: upstream,
	}
	if task.Tool != "" {
		if tool, ok := r.options.Tools[task.Tool]; ok {
			entry := tool.Entry
			context.Tool = &entry
		}
	}
	return context
}

func (r *Runner) setUpstream(name string, digest string) {
	r.mu.Lock()
	r.upstream[name] = digest
	r.mu.Unlock()
}

func cloneMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}
