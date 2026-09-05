package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

const taskCommandBuildFile = `version = 1

[tasks.prepare]
type = "go"
args = ["version"]

[tasks.assets]
type = "group"
depends-on = ["prepare"]
`

func TestCommand_whenTasksRequested_shouldListStableTaskMetadataWithoutProcesses(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"goark.build": taskCommandBuildFile,
	})
	runner := &recordingProcessRunner{}
	var stdout bytes.Buffer
	command := Command{Dir: root, Out: &stdout, Err: io.Discard, Runner: runner}
	if code := command.Run([]string{"tasks", "--json"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	var tasks []struct {
		Name      string   `json:"name"`
		Type      string   `json:"type"`
		DependsOn []string `json:"dependsOn"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &tasks); err != nil {
		t.Fatalf("JSON 无效: %v\n%s", err, stdout.String())
	}
	if len(tasks) != 2 || tasks[0].Name != "assets" || tasks[1].Name != "prepare" || tasks[0].DependsOn[0] != "prepare" {
		t.Fatalf("任务列表错误: %#v", tasks)
	}
	if len(runner.requests) != 0 {
		t.Fatalf("任务查询不应启动进程: %#v", runner.requests)
	}
}

func TestCommand_whenGraphFormatRequested_shouldRenderStableGraph(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"goark.build": taskCommandBuildFile,
	})
	tests := []struct {
		format string
		want   string
	}{
		{format: "text", want: "assets -> prepare\nprepare\n"},
		{format: "json", want: `"name":"assets"`},
		{format: "dot", want: `"prepare" -> "assets"`},
	}
	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			var stdout bytes.Buffer
			command := Command{Dir: root, Out: &stdout, Err: io.Discard, Runner: &recordingProcessRunner{}}
			if code := command.Run([]string{"graph", "--format=" + test.format}); code != 0 {
				t.Fatalf("退出码 = %d", code)
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("图输出缺少 %q:\n%s", test.want, stdout.String())
			}
		})
	}
}

func TestCommand_whenTaskDryRunRequested_shouldExecuteDependenciesInOrderWithoutProcesses(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"goark.build": taskCommandBuildFile,
	})
	runner := &recordingProcessRunner{}
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: io.Discard, Err: &stderr, Runner: runner}
	if code := command.Run([]string{"task", "assets", "--goark-dry-run"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	assertOrderedFragments(t, stderr.String(), []string{"would run task prepare", "would run task assets"})
	if len(runner.requests) != 0 {
		t.Fatalf("模拟执行不应启动进程: %#v", runner.requests)
	}
}

func TestCommand_whenUnknownGraphFormatRequested_shouldReturnUsageError(t *testing.T) {
	root := writeTestModule(t, map[string]string{"go.mod": "module example.com/app\n\ngo 1.25\n"})
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: io.Discard, Err: &stderr, Runner: &recordingProcessRunner{}}
	if code := command.Run([]string{"graph", "--format=svg"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
	if !strings.Contains(stderr.String(), "text、json、dot") {
		t.Fatalf("错误信息不完整: %q", stderr.String())
	}
}

func TestCommand_whenTasksContainOutputConflict_shouldRejectBeforeRendering(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.25\n",
		"goark.build": `version = 1
[tasks.one]
type = "delete"
outputs = ["build"]
[tasks.two]
type = "delete"
outputs = ["build/app"]
`,
	})
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: io.Discard, Err: &stderr, Runner: &recordingProcessRunner{}}
	if code := command.Run([]string{"tasks"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
	if !strings.Contains(stderr.String(), "输出冲突") {
		t.Fatalf("错误信息不完整: %q", stderr.String())
	}
}

func TestCommand_whenTaskDoesNotUseUnavailableTool_shouldNotResolveIt(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.25\n",
		"goark.build": `version = 1
[tools.missing]
type = "system"
command = "goark-command-that-does-not-exist"
install = "manual"
[tasks.prepare]
type = "go"
args = ["version"]
`,
	})
	runner := &recordingProcessRunner{}
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: io.Discard, Err: &stderr, Runner: runner}
	if code := command.Run([]string{"task", "prepare", "--goark-dry-run"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if len(runner.requests) != 0 {
		t.Fatalf("模拟执行不应启动进程: %#v", runner.requests)
	}
}
