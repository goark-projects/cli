package taskcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/envutil"
	"goark.dev/cli/internal/toollock"
)

// Context 包含缓存指纹所需的全部显式输入。
type Context struct {
	Root        string
	TaskName    string
	Task        buildspec.Task
	Tool        *toollock.Entry
	GoVersion   string
	GOOS        string
	GOARCH      string
	BuildTags   []string
	Profile     string
	Environment map[string]string
	Upstream    map[string]string
}

type fingerprintModel struct {
	TaskName    string            `json:"taskName"`
	Task        buildspec.Task    `json:"task"`
	Inputs      []fileDigest      `json:"inputs"`
	Tool        *toollock.Entry   `json:"tool,omitempty"`
	GoVersion   string            `json:"goVersion"`
	GOOS        string            `json:"goos"`
	GOARCH      string            `json:"goarch"`
	BuildTags   []string          `json:"buildTags"`
	Profile     string            `json:"profile"`
	Environment []environmentHash `json:"environment"`
	Upstream    map[string]string `json:"upstream"`
}

type environmentHash struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	SHA256  string `json:"sha256"`
}

// Fingerprint 计算任务缓存键，不持久化环境变量明文。
func Fingerprint(context Context) (string, error) {
	inputs, err := collect(context.Root, context.Task.Inputs, false)
	if err != nil {
		return "", fmt.Errorf("收集任务 %q inputs 失败: %w", context.TaskName, err)
	}
	environment := make([]environmentHash, 0, len(context.Task.EnvironmentInputs))
	for _, name := range context.Task.EnvironmentInputs {
		value, present := envutil.Lookup(context.Environment, name)
		environment = append(environment, environmentHash{Name: name, Present: present, SHA256: hashBytes([]byte(value))})
	}
	sort.Slice(environment, func(left int, right int) bool { return environment[left].Name < environment[right].Name })
	buildTags := append([]string(nil), context.BuildTags...)
	sort.Strings(buildTags)
	model := fingerprintModel{
		TaskName: context.TaskName, Task: context.Task, Inputs: inputs, Tool: context.Tool,
		GoVersion: context.GoVersion, GOOS: context.GOOS, GOARCH: context.GOARCH,
		BuildTags: buildTags, Profile: context.Profile, Environment: environment,
		Upstream: cloneMap(context.Upstream),
	}
	data, err := json.Marshal(model)
	if err != nil {
		return "", fmt.Errorf("编码任务缓存指纹失败: %w", err)
	}
	return hashBytes(data), nil
}

func hashBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func cloneMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}
