package buildspec

import "time"

const (
	// FileName 是项目描述文件的固定名称。
	FileName = "goark.build"
	// LockFileName 是项目工具锁文件的固定名称。
	LockFileName = "goark.build.lock"
	// CurrentVersion 是当前支持的项目描述格式版本。
	CurrentVersion = 1
)

// TaskType 表示任务执行模型。
type TaskType string

const (
	TaskTypeExec          TaskType = "exec"
	TaskTypeGoarkGenerate TaskType = "goark-generate"
	TaskTypeGo            TaskType = "go"
	TaskTypeDelete        TaskType = "delete"
	TaskTypeGroup         TaskType = "group"
)

// ToolType 表示外部工具的来源。
type ToolType string

const (
	ToolTypeGo     ToolType = "go"
	ToolTypeSystem ToolType = "system"
	ToolTypeLocal  ToolType = "local"
)

// Document 是 goark.build 的完整内存模型。
type Document struct {
	Version   int                `toml:"version" json:"version"`
	Project   Project            `toml:"project" json:"project"`
	Execution Execution          `toml:"execution" json:"execution"`
	Generate  Generate           `toml:"generate" json:"generate"`
	Commands  map[string]Command `toml:"commands" json:"commands"`
	Tools     map[string]Tool    `toml:"tools" json:"tools"`
	Tasks     map[string]Task    `toml:"tasks" json:"tasks"`
	Profiles  map[string]Profile `toml:"profiles" json:"profiles"`
}

// Project 描述项目自身信息，模块路径与 Go 版本仍由 go.mod 提供。
type Project struct {
	Name        string `toml:"name" json:"name"`
	Main        string `toml:"main" json:"main"`
	Description string `toml:"description" json:"description,omitempty"`
}

// Execution 描述任务调度的全局边界。
type Execution struct {
	MaxParallel    int      `toml:"max-parallel" json:"maxParallel"`
	FailFast       bool     `toml:"fail-fast" json:"failFast"`
	DefaultTimeout Duration `toml:"default-timeout" json:"defaultTimeout"`
}

// Generate 描述 Goark 内置代码生成行为。
type Generate struct {
	Patterns   []string `toml:"patterns" json:"patterns"`
	CleanStale bool     `toml:"clean-stale" json:"cleanStale"`
}

// Command 描述一个固定 CLI 命令的生命周期扩展。
type Command struct {
	Before          []string          `toml:"before" json:"before,omitempty"`
	After           []string          `toml:"after" json:"after,omitempty"`
	Finally         []string          `toml:"finally" json:"finally,omitempty"`
	GoArgs          []string          `toml:"go-args" json:"goArgs,omitempty"`
	ApplicationArgs []string          `toml:"application-args" json:"applicationArgs,omitempty"`
	Environment     map[string]string `toml:"environment" json:"environment,omitempty"`
	Output          string            `toml:"output" json:"output,omitempty"`
}

// Tool 描述任务使用的外部工具。
type Tool struct {
	Type    ToolType `toml:"type" json:"type"`
	Package string   `toml:"package" json:"package,omitempty"`
	Version string   `toml:"version" json:"version,omitempty"`
	Install string   `toml:"install" json:"install"`
	Command string   `toml:"command" json:"command,omitempty"`
	Path    string   `toml:"path" json:"path,omitempty"`
}

// Task 描述任务图中的一个节点。
type Task struct {
	Type              TaskType          `toml:"type" json:"type"`
	Tool              string            `toml:"tool" json:"tool,omitempty"`
	Args              []string          `toml:"args" json:"args,omitempty"`
	WorkingDirectory  string            `toml:"working-directory" json:"workingDirectory,omitempty"`
	DependsOn         []string          `toml:"depends-on" json:"dependsOn,omitempty"`
	Inputs            []string          `toml:"inputs" json:"inputs,omitempty"`
	Outputs           []string          `toml:"outputs" json:"outputs,omitempty"`
	EnvironmentInputs []string          `toml:"environment-inputs" json:"environmentInputs,omitempty"`
	Environment       map[string]string `toml:"environment" json:"environment,omitempty"`
	Timeout           Duration          `toml:"timeout" json:"timeout,omitempty"`
	Cache             bool              `toml:"cache" json:"cache"`
	ParallelSafe      bool              `toml:"parallel-safe" json:"parallelSafe"`
	When              string            `toml:"when" json:"when,omitempty"`
}

// Profile 描述构建 Profile 对参数和环境的覆盖。
type Profile struct {
	GoArgs          []string          `toml:"go-args" json:"goArgs,omitempty"`
	ApplicationArgs []string          `toml:"application-args" json:"applicationArgs,omitempty"`
	Environment     map[string]string `toml:"environment" json:"environment,omitempty"`
}

// Duration 为 TOML 持续时间提供强类型解析。
type Duration struct {
	time.Duration
}

// UnmarshalText 按 Go duration 语法解析配置值。
func (d *Duration) UnmarshalText(text []byte) error {
	value, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = value
	return nil
}

// MarshalText 输出稳定的 Go duration 字符串。
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}
