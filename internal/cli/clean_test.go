package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_whenCleanRequested_shouldDeleteDeclaredOutputsAndCache(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"goark.build": "version = 1\n[commands.build]\noutput = \"build/app\"\n",
		"build/app":   "binary",
		"keep.txt":    "keep",
	})
	if err := os.MkdirAll(filepath.Join(root, ".goark", "cache"), 0o755); err != nil {
		t.Fatalf("创建缓存失败: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: &stdout, Err: &stderr, Runner: &recordingProcessRunner{}}
	if code := command.Run([]string{"clean"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "build", "app")); !os.IsNotExist(err) {
		t.Fatalf("构建输出未删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "keep.txt")); err != nil {
		t.Fatalf("未声明文件被删除: %v", err)
	}
}

func TestCommand_whenCleanDryRunRequested_shouldReportWithoutDeletingOrProcesses(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"goark.build": "version = 1\n[commands.build]\noutput = \"build/app\"\n",
		"build/app":   "binary",
	})
	runner := &recordingProcessRunner{}
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: io.Discard, Err: &stderr, Runner: runner}
	if code := command.Run([]string{"clean", "--goark-dry-run"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "build", "app")); err != nil {
		t.Fatalf("模拟清理删除了输出: %v", err)
	}
	if len(runner.requests) != 0 || !strings.Contains(stderr.String(), "would remove build/app") {
		t.Fatalf("模拟清理不正确: requests=%#v stderr=%q", runner.requests, stderr.String())
	}
}
