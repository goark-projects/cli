package buildspec

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var supportedCommands = map[string]struct{}{
	"generate": {},
	"run":      {},
	"build":    {},
	"test":     {},
	"install":  {},
	"vet":      {},
	"list":     {},
	"fix":      {},
}

func validateDocument(document Document) error {
	if document.Version != CurrentVersion {
		if document.Version == 0 {
			return fmt.Errorf("version = 1 为必填字段")
		}
		return fmt.Errorf("不支持 version = %d，仅支持 version = %d", document.Version, CurrentVersion)
	}
	if document.Execution.MaxParallel < 1 {
		return fmt.Errorf("execution.max-parallel 必须大于零")
	}
	if document.Execution.DefaultTimeout.Duration <= 0 {
		return fmt.Errorf("execution.default-timeout 必须大于零")
	}
	if err := validateProject(document.Project); err != nil {
		return err
	}
	if err := validateGenerate(document.Generate); err != nil {
		return err
	}
	if err := validateTools(document.Tools); err != nil {
		return err
	}
	if err := validateTasks(document.Tasks, document.Tools); err != nil {
		return err
	}
	if err := validateCommands(document.Commands, document.Tasks); err != nil {
		return err
	}
	return validateProfiles(document.Profiles)
}

func validateGenerate(generate Generate) error {
	if len(generate.Patterns) == 0 {
		return fmt.Errorf("generate.patterns 至少需要一个项目内路径")
	}
	for _, pattern := range generate.Patterns {
		normalized := strings.ReplaceAll(strings.TrimSpace(pattern), "\\", "/")
		if normalized != "." && !strings.HasPrefix(normalized, "./") {
			return fmt.Errorf("generate.patterns 必须使用 . 或 ./ 开头的项目内路径: %q", pattern)
		}
		clean := path.Clean(normalized)
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("generate.patterns 不能逃出项目根目录: %q", pattern)
		}
	}
	return nil
}

func validateProject(project Project) error {
	if project.Main != "" {
		if err := validateProjectPath("project.main", project.Main); err != nil {
			return err
		}
	}
	return nil
}

func validateTools(tools map[string]Tool) error {
	for _, name := range sortedKeys(tools) {
		tool := tools[name]
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("工具名称 %q 无效", name)
		}
		switch tool.Type {
		case ToolTypeGo:
			if tool.Package == "" || tool.Version == "" {
				return fmt.Errorf("Go 工具 %q 必须声明 package 和 version", name)
			}
			if tool.Install != "" && tool.Install != "auto" && tool.Install != "manual" {
				return fmt.Errorf("工具 %q 的 install 必须是 auto 或 manual", name)
			}
		case ToolTypeSystem:
			if tool.Command == "" {
				return fmt.Errorf("系统工具 %q 必须声明 command", name)
			}
			if tool.Install != "" && tool.Install != "manual" {
				return fmt.Errorf("系统工具 %q 仅支持 install = \"manual\"", name)
			}
		case ToolTypeLocal:
			if tool.Path == "" {
				return fmt.Errorf("本地工具 %q 必须声明 path", name)
			}
			if err := validateProjectPath("tools."+name+".path", tool.Path); err != nil {
				return err
			}
			if tool.Install != "" && tool.Install != "manual" {
				return fmt.Errorf("本地工具 %q 仅支持 install = \"manual\"", name)
			}
		default:
			return fmt.Errorf("工具 %q 的类型 %q 无效", name, tool.Type)
		}
	}
	return nil
}

func validateTasks(tasks map[string]Task, tools map[string]Tool) error {
	for _, name := range sortedKeys(tasks) {
		task := tasks[name]
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("任务名称 %q 无效", name)
		}
		switch task.Type {
		case TaskTypeExec:
			if _, ok := tools[task.Tool]; !ok {
				return fmt.Errorf("任务 %q 引用了不存在的工具 %q", name, task.Tool)
			}
		case TaskTypeGo:
			if len(task.Args) == 0 {
				return fmt.Errorf("Go 任务 %q 必须声明 args", name)
			}
		case TaskTypeDelete:
			if len(task.Outputs) == 0 {
				return fmt.Errorf("删除任务 %q 必须声明 outputs", name)
			}
			if task.Cache {
				return fmt.Errorf("删除任务 %q 不能启用 cache", name)
			}
		case TaskTypeGroup:
			if len(task.DependsOn) == 0 {
				return fmt.Errorf("聚合任务 %q 必须声明 depends-on", name)
			}
		case TaskTypeGoarkGenerate:
			return fmt.Errorf("任务 %q 使用了仅供 CLI 内部使用的任务类型 %q", name, task.Type)
		default:
			return fmt.Errorf("任务 %q 的任务类型 %q 无效", name, task.Type)
		}
		if task.Timeout.Duration < 0 {
			return fmt.Errorf("任务 %q 的 timeout 不能小于零", name)
		}
		if task.Cache && len(task.Inputs) == 0 {
			return fmt.Errorf("缓存任务 %q 必须声明 inputs", name)
		}
		if task.Cache && len(task.Outputs) == 0 {
			return fmt.Errorf("缓存任务 %q 必须声明 outputs", name)
		}
		if task.WorkingDirectory != "" {
			if err := validateProjectPath("tasks."+name+".working-directory", task.WorkingDirectory); err != nil {
				return err
			}
		}
		for _, input := range task.Inputs {
			if err := validateProjectPath("tasks."+name+".inputs", input); err != nil {
				return err
			}
		}
		for _, output := range task.Outputs {
			if err := validateProjectPath("tasks."+name+".outputs", output); err != nil {
				return err
			}
		}
		for _, environmentName := range task.EnvironmentInputs {
			if !environmentNamePattern.MatchString(environmentName) {
				return fmt.Errorf("任务 %q 的 environment-inputs 名称 %q 无效", name, environmentName)
			}
		}
		if err := validateEnvironment(fmt.Sprintf("任务 %q", name), task.Environment); err != nil {
			return err
		}
		for _, dependency := range task.DependsOn {
			if _, ok := tasks[dependency]; !ok {
				return fmt.Errorf("任务 %q 依赖不存在的任务 %q", name, dependency)
			}
		}
	}
	return nil
}

func validateCommands(commands map[string]Command, tasks map[string]Task) error {
	for _, name := range sortedKeys(commands) {
		command := commands[name]
		if _, ok := supportedCommands[name]; !ok {
			return fmt.Errorf("未知命令配置 %q", name)
		}
		for _, taskName := range append(append(append([]string(nil), command.Before...), command.After...), command.Finally...) {
			if _, ok := tasks[taskName]; !ok {
				return fmt.Errorf("命令 %q 引用了不存在的任务 %q", name, taskName)
			}
		}
		if command.Output != "" {
			if err := validateProjectPath("commands."+name+".output", command.Output); err != nil {
				return err
			}
		}
		if err := validateEnvironment(fmt.Sprintf("命令 %q", name), command.Environment); err != nil {
			return err
		}
	}
	return nil
}

func validateProfiles(profiles map[string]Profile) error {
	for _, name := range sortedKeys(profiles) {
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("Profile 名称 %q 无效", name)
		}
		if err := validateEnvironment(fmt.Sprintf("Profile %q", name), profiles[name].Environment); err != nil {
			return err
		}
	}
	return nil
}

func validateEnvironment(owner string, environment map[string]string) error {
	seen := make(map[string]string, len(environment))
	for _, name := range sortedKeys(environment) {
		if !environmentNamePattern.MatchString(name) {
			return fmt.Errorf("%s 的 environment 名称 %q 无效", owner, name)
		}
		canonical := strings.ToUpper(name)
		if previous, ok := seen[canonical]; ok {
			return fmt.Errorf("%s 的 environment 名称 %q 与 %q 重复", owner, previous, name)
		}
		seen[canonical] = name
	}
	return nil
}

func validateProjectPath(field string, value string) error {
	if filepath.IsAbs(value) {
		return fmt.Errorf("%s 必须位于项目内，不能使用绝对路径 %q", field, value)
	}
	clean := filepath.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s 不能逃出项目根目录: %q", field, value)
	}
	return nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
