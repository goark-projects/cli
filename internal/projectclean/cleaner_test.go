package projectclean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
)

func TestCleanerRun_whenOutputsDeclared_shouldDeleteOnlyOutputsAndCache(t *testing.T) {
	root := t.TempDir()
	writeCleanerFile(t, root, "build/app", "binary")
	writeCleanerFile(t, root, "generated/one.txt", "generated")
	writeCleanerFile(t, root, ".goark/cache/tasks/item.json", "cache")
	writeCleanerFile(t, root, "keep.txt", "keep")
	document := buildspec.Document{
		Commands: map[string]buildspec.Command{"build": {Output: "build/app"}},
		Tasks:    map[string]buildspec.Task{"generate": {Outputs: []string{"generated/*.txt"}}},
	}
	removed, err := (Cleaner{Root: root, Document: document}).Run(false)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if len(removed) != 3 {
		t.Fatalf("清理项 = %#v", removed)
	}
	for _, name := range []string{"build/app", "generated/one.txt", ".goark/cache"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("%s 未删除: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "keep.txt")); err != nil {
		t.Fatalf("未声明文件被删除: %v", err)
	}
}

func TestCleanerRun_whenDryRunRequested_shouldNotDelete(t *testing.T) {
	root := t.TempDir()
	writeCleanerFile(t, root, "build/app", "binary")
	document := buildspec.Document{Commands: map[string]buildspec.Command{"build": {Output: "build/app"}}}
	removed, err := (Cleaner{Root: root, Document: document}).Run(true)
	if err != nil || len(removed) != 1 {
		t.Fatalf("模拟清理 = %#v, err=%v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "app")); err != nil {
		t.Fatalf("模拟清理删除了输出: %v", err)
	}
}

func TestCleanerRun_whenOutputSymlinkEscapesProject_shouldReject(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}
	writeCleanerFile(t, outside, "keep.txt", "keep")
	if err := os.Symlink(outside, filepath.Join(root, "output")); err != nil {
		t.Skipf("当前环境无法创建符号链接: %v", err)
	}
	document := buildspec.Document{Commands: map[string]buildspec.Command{"build": {Output: "output"}}}
	_, err := (Cleaner{Root: root, Document: document}).Run(false)
	if err == nil || !strings.Contains(err.Error(), "符号链接") {
		t.Fatalf("错误 = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "keep.txt")); err != nil {
		t.Fatalf("项目外文件被删除: %v", err)
	}
}

func writeCleanerFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
}
