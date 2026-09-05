package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/projectfs"
	"goark.dev/cli/internal/taskcache"
	"goark.dev/cli/internal/taskgraph"
	"goark.dev/cli/internal/taskrunner"
	"goark.dev/cli/internal/tooling"
	"goark.dev/cli/internal/toollock"
)

type preparedLifecycle struct {
	command    Command
	project    goarkProject
	plan       buildplan.Plan
	graph      *taskgraph.Graph
	taskRunner *taskrunner.Runner
}

func (c Command) prepareLifecycle(project goarkProject, plan buildplan.Plan) (*preparedLifecycle, error) {
	graph, err := taskgraph.New(project.Build.Tasks)
	if err != nil {
		return nil, err
	}
	if err := validateTaskPaths(project); err != nil {
		return nil, err
	}
	tools, err := c.resolveLifecycleTools(project, plan)
	if err != nil {
		return nil, err
	}
	overrides := lifecycleOverrides(project.Build, plan)
	goVersion := "unavailable"
	if !plan.Control.DryRun {
		goVersion = c.captureGoVersion()
	}
	runner := taskrunner.New(taskrunner.Options{
		Root: project.Root, ProjectName: project.ProjectName(), ProjectModule: project.ModulePath,
		Profile: plan.Profile, CommandOutput: plan.Output, Environment: plan.Environment,
		OverrideEnvironment: overrides, Tools: tools, Process: c.Runner,
		Cache: taskcache.NewStore(project.Root), In: c.In, Out: c.Out, Err: c.Err,
		DefaultTimeout: project.Build.Execution.DefaultTimeout.Duration, DryRun: plan.Control.DryRun,
		GoVersion: goVersion, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		BuildTags: buildTags(plan.GoArguments),
	})
	return &preparedLifecycle{command: c, project: project, plan: plan, graph: graph, taskRunner: runner}, nil
}

func (c Command) resolveLifecycleTools(project goarkProject, plan buildplan.Plan) (map[string]tooling.Resolved, error) {
	resolved := make(map[string]tooling.Resolved, len(project.Build.Tools))
	if len(project.Build.Tools) == 0 {
		return resolved, nil
	}
	lock, err := toollock.Read(project.Root)
	if err != nil {
		return nil, fmt.Errorf("工具锁文件不可用，请先执行 goark sync: %w", err)
	}
	digest, err := toollock.DigestFile(filepath.Join(project.Root, buildspec.FileName))
	if err != nil {
		return nil, err
	}
	if err := lock.VerifyBuild(digest); err != nil {
		return nil, err
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("解析用户缓存目录失败: %w", err)
	}
	manager := tooling.NewManager(project.Root, filepath.Join(cacheRoot, "goark", "tools"), plan.Environment)
	names := make([]string, 0, len(project.Build.Tools))
	for name := range project.Build.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tool := project.Build.Tools[name]
		locked, ok := lock.Find(name, runtime.GOOS, runtime.GOARCH)
		if !ok {
			return nil, fmt.Errorf("工具 %q 缺少 %s/%s 锁定项", name, runtime.GOOS, runtime.GOARCH)
		}
		if !lockMatchesDeclaration(locked, tool) {
			return nil, fmt.Errorf("工具 %q 的声明与锁定项不一致", name)
		}
		// 自动恢复必须等待项目信任记录；普通生命周期当前只消费已安装且锁定的工具。
		allowInstall := false
		item, err := manager.Resolve(c.Context, name, tool, tooling.ResolveOptions{AllowInstall: allowInstall, Offline: plan.Control.Offline})
		if err != nil {
			return nil, err
		}
		if err := tooling.Verify(item, locked); err != nil {
			return nil, err
		}
		resolved[name] = item
	}
	return resolved, nil
}

func validateTaskPaths(project goarkProject) error {
	resolver := projectfs.New(project.Root)
	for name, task := range project.Build.Tasks {
		if task.WorkingDirectory != "" {
			if _, err := resolver.Resolve(task.WorkingDirectory, projectfs.MustExist); err != nil {
				return fmt.Errorf("任务 %q 的 working-directory 无效: %w", name, err)
			}
		}
		for _, pattern := range append(append([]string(nil), task.Inputs...), task.Outputs...) {
			if strings.Contains(pattern, "${") {
				continue
			}
			if _, err := resolver.ResolvePattern(pattern); err != nil {
				return fmt.Errorf("任务 %q 的路径无效: %w", name, err)
			}
		}
	}
	return nil
}

func lifecycleOverrides(document buildspec.Document, plan buildplan.Plan) map[string]string {
	result := make(map[string]string)
	for name, value := range document.Commands[plan.Command].Environment {
		result[name] = value
	}
	if profile, ok := document.Profiles[plan.Profile]; ok {
		for name, value := range profile.Environment {
			result[name] = value
		}
	}
	for name, value := range plan.Control.Environment {
		result[name] = value
	}
	return result
}

func lockMatchesDeclaration(entry toollock.Entry, tool buildspec.Tool) bool {
	return entry.Type == tool.Type && entry.Package == tool.Package && entry.Version == tool.Version
}

func buildTags(arguments []string) []string {
	var tags []string
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		value := ""
		switch {
		case strings.HasPrefix(argument, "-tags="):
			value = strings.TrimPrefix(argument, "-tags=")
		case argument == "-tags" && index+1 < len(arguments):
			index++
			value = arguments[index]
		}
		if value != "" {
			tags = append(tags, strings.FieldsFunc(value, func(char rune) bool { return char == ',' || char == ' ' })...)
		}
	}
	sort.Strings(tags)
	return tags
}
