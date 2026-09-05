package taskcache

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/toollock"
)

func TestFingerprint_whenDeclaredInputChanges_shouldChange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "input/source.go", "package input\n")
	context := Context{
		Root:        root,
		TaskName:    "generate",
		Task:        buildspec.Task{Type: buildspec.TaskTypeExec, Inputs: []string{"input/**/*.go"}, Outputs: []string{"output/*.go"}, EnvironmentInputs: []string{"CONFIG"}},
		Tool:        &toollock.Entry{Name: "generator", SHA256: strings.Repeat("a", 64)},
		GoVersion:   "go1.27.0",
		GOOS:        "linux",
		GOARCH:      "amd64",
		BuildTags:   []string{"integration", "production"},
		Profile:     "production",
		Environment: map[string]string{"CONFIG": "first", "UNDECLARED": "ignored"},
		Upstream:    map[string]string{"prepare": strings.Repeat("b", 64)},
	}
	first, err := Fingerprint(context)
	if err != nil {
		t.Fatalf("计算初始指纹失败: %v", err)
	}
	writeFile(t, root, "input/source.go", "package changed\n")
	second, err := Fingerprint(context)
	if err != nil {
		t.Fatalf("计算变更指纹失败: %v", err)
	}
	if first == second {
		t.Fatal("输入内容变化后指纹未变化")
	}

	context.Environment["CONFIG"] = "second"
	third, err := Fingerprint(context)
	if err != nil {
		t.Fatalf("计算环境指纹失败: %v", err)
	}
	if second == third {
		t.Fatal("声明环境变化后指纹未变化")
	}
}

func TestFingerprint_whenWindowsEnvironmentNameUsesDifferentCase_shouldTrackValue(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅适用于 Windows 环境名语义")
	}
	context := Context{
		Root: t.TempDir(), TaskName: "environment",
		Task:        buildspec.Task{EnvironmentInputs: []string{"PATH"}},
		Environment: map[string]string{"Path": "first"},
	}
	first, err := Fingerprint(context)
	if err != nil {
		t.Fatalf("计算初始指纹失败: %v", err)
	}
	context.Environment["Path"] = "second"
	second, err := Fingerprint(context)
	if err != nil {
		t.Fatalf("计算变更指纹失败: %v", err)
	}
	if first == second {
		t.Fatal("Windows 环境值变化后指纹未变化")
	}
}

func TestStore_whenOutputsRemainIdentical_shouldHitCache(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "input/source.txt", "input")
	writeFile(t, root, "output/result.txt", "result")
	context := Context{
		Root: root, TaskName: "copy",
		Task: buildspec.Task{Inputs: []string{"input/*.txt"}, Outputs: []string{"output/*.txt"}},
	}
	store := NewStore(root)
	if err := store.Save(context); err != nil {
		t.Fatalf("保存缓存失败: %v", err)
	}
	hit, err := store.Lookup(context)
	if err != nil || !hit {
		t.Fatalf("缓存命中 = %t, err=%v", hit, err)
	}

	writeFile(t, root, "output/result.txt", "changed")
	hit, err = store.Lookup(context)
	if err != nil {
		t.Fatalf("重新校验缓存失败: %v", err)
	}
	if hit {
		t.Fatal("输出摘要变化后不应命中缓存")
	}
}

func TestStoreSave_whenOutputDoesNotExist_shouldReject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "input/source.txt", "input")
	store := NewStore(root)
	err := store.Save(Context{
		Root: root, TaskName: "missing",
		Task: buildspec.Task{Inputs: []string{"input/*.txt"}, Outputs: []string{"output/*.txt"}},
	})
	if err == nil || !strings.Contains(err.Error(), "outputs") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestStoreLookup_whenManifestIsCorrupted_shouldMissWithoutFailingTask(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "input/source.txt", "input")
	writeFile(t, root, "output/result.txt", "result")
	context := Context{
		Root: root, TaskName: "copy",
		Task: buildspec.Task{Inputs: []string{"input/*.txt"}, Outputs: []string{"output/*.txt"}},
	}
	store := NewStore(root)
	fingerprint, err := Fingerprint(context)
	if err != nil {
		t.Fatalf("计算缓存指纹失败: %v", err)
	}
	writeFile(t, root, filepath.ToSlash(filepath.Join(".goark", "cache", "tasks", "copy", fingerprint+".json")), "{broken")

	hit, err := store.Lookup(context)
	if err != nil {
		t.Fatalf("损坏缓存清单不应阻断任务: %v", err)
	}
	if hit {
		t.Fatal("损坏缓存清单不应命中")
	}
}

func TestOutputDigest_whenDirectoryContainsFiles_shouldBeStable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "output/a.txt", "a")
	writeFile(t, root, "output/nested/b.txt", "b")
	context := Context{Root: root, Task: buildspec.Task{Outputs: []string{"output"}}}
	first, err := OutputDigest(context)
	if err != nil {
		t.Fatalf("计算输出摘要失败: %v", err)
	}
	second, err := OutputDigest(context)
	if err != nil {
		t.Fatalf("重复计算输出摘要失败: %v", err)
	}
	if first == "" || first != second {
		t.Fatalf("输出摘要不稳定: %q %q", first, second)
	}
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
}
