package taskview

import (
	"bytes"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
)

func TestSnapshot_whenTasksUnordered_shouldReturnCopiedStableOrder(t *testing.T) {
	tasks := map[string]buildspec.Task{
		"prepare": {Type: buildspec.TaskTypeGo},
		"assets":  {Type: buildspec.TaskTypeGroup, DependsOn: []string{"prepare"}},
	}
	snapshot := Snapshot(tasks)
	tasks["assets"] = buildspec.Task{Type: buildspec.TaskTypeDelete}
	if len(snapshot) != 2 || snapshot[0].Name != "assets" || snapshot[0].DependsOn[0] != "prepare" || snapshot[1].Name != "prepare" {
		t.Fatalf("任务快照错误: %#v", snapshot)
	}
}

func TestWriteGraph_whenFormatsRequested_shouldUseStableDependencyDirection(t *testing.T) {
	tasks := []Task{
		{Name: "assets", Type: buildspec.TaskTypeGroup, DependsOn: []string{"prepare"}},
		{Name: "prepare", Type: buildspec.TaskTypeGo},
	}
	tests := []struct {
		format Format
		want   string
	}{
		{format: FormatText, want: "assets -> prepare\nprepare\n"},
		{format: FormatJSON, want: `"dependsOn":["prepare"]`},
		{format: FormatDOT, want: `"prepare" -> "assets";`},
	}
	for _, test := range tests {
		t.Run(string(test.format), func(t *testing.T) {
			var output bytes.Buffer
			if err := WriteGraph(&output, tasks, test.format); err != nil {
				t.Fatalf("输出任务图失败: %v", err)
			}
			if !strings.Contains(output.String(), test.want) {
				t.Fatalf("输出缺少 %q:\n%s", test.want, output.String())
			}
		})
	}
}
