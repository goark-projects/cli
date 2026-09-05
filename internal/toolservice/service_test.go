package toolservice

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/projecttrust"
	"goark.dev/cli/internal/tooling"
	"goark.dev/cli/internal/toollock"
)

func TestServiceSync_whenGoToolIsManual_shouldRequireExplicitInstall(t *testing.T) {
	root, document := writeGoToolProject(t, "manual")
	installCount := 0
	service := fakeGoToolService(t, root, document, &installCount)
	if _, err := service.Sync(context.Background(), SyncOptions{}); err == nil || !strings.Contains(err.Error(), "尚未安装") {
		t.Fatalf("错误 = %v", err)
	}
	if installCount != 0 {
		t.Fatalf("普通同步安装了 manual 工具: %d", installCount)
	}
	if _, err := service.Install(context.Background(), "demo", false); err != nil {
		t.Fatalf("显式安装失败: %v", err)
	}
	if installCount != 1 {
		t.Fatalf("显式安装次数 = %d", installCount)
	}
}

func TestServiceSync_whenGoToolIsAuto_shouldInstallUnlessOffline(t *testing.T) {
	root, document := writeGoToolProject(t, "auto")
	installCount := 0
	service := fakeGoToolService(t, root, document, &installCount)
	if _, err := service.Sync(context.Background(), SyncOptions{Offline: true}); err == nil || !strings.Contains(err.Error(), "离线") {
		t.Fatalf("离线错误 = %v", err)
	}
	if installCount != 0 {
		t.Fatalf("离线同步启动了安装: %d", installCount)
	}
	if _, err := service.Sync(context.Background(), SyncOptions{}); err != nil {
		t.Fatalf("自动安装同步失败: %v", err)
	}
	if installCount != 1 {
		t.Fatalf("自动安装次数 = %d", installCount)
	}
}

func TestServiceSync_whenToolsResolve_shouldWriteStableLockAndTrustProject(t *testing.T) {
	root, document := writeToolProject(t)
	trust := projecttrust.Store{Dir: t.TempDir()}
	service := newTestService(root, document, trust)
	if _, err := service.Sync(context.Background(), SyncOptions{}); err != nil {
		t.Fatalf("同步工具失败: %v", err)
	}
	path := filepath.Join(root, buildspec.LockFileName)
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取锁文件失败: %v", err)
	}
	if _, err := service.Sync(context.Background(), SyncOptions{}); err != nil {
		t.Fatalf("重复同步工具失败: %v", err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatalf("锁文件输出不稳定:\n%s\n---\n%s", first, second)
	}
	digest, _ := toollock.DigestFile(filepath.Join(root, buildspec.FileName))
	if err := trust.Verify(root, digest); err != nil {
		t.Fatalf("项目未建立信任: %v", err)
	}
}

func TestServiceSync_whenLocked_shouldVerifyWithoutWriting(t *testing.T) {
	root, document := writeToolProject(t)
	service := newTestService(root, document, projecttrust.Store{Dir: t.TempDir()})
	if _, err := service.Sync(context.Background(), SyncOptions{}); err != nil {
		t.Fatalf("准备锁文件失败: %v", err)
	}
	path := filepath.Join(root, buildspec.LockFileName)
	before, _ := os.Stat(path)
	if _, err := service.Sync(context.Background(), SyncOptions{Locked: true}); err != nil {
		t.Fatalf("锁定验证失败: %v", err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("--locked 不应更新锁文件")
	}
}

func TestServiceSync_whenOtherPlatformEntriesExist_shouldPreserveThem(t *testing.T) {
	root, document := writeToolProject(t)
	digest, _ := toollock.DigestFile(filepath.Join(root, buildspec.FileName))
	otherOS := "linux"
	if runtime.GOOS == otherOS {
		otherOS = "windows"
	}
	old := toollock.Entry{
		Name: "go", Type: buildspec.ToolTypeSystem, GOOS: otherOS, GOARCH: "arm64",
		Path: "/usr/bin/go", SHA256: strings.Repeat("a", 64),
	}
	if err := toollock.Write(root, toollock.File{Version: toollock.CurrentVersion, BuildSHA256: digest, Tools: []toollock.Entry{old}}); err != nil {
		t.Fatalf("准备跨平台锁文件失败: %v", err)
	}
	service := newTestService(root, document, projecttrust.Store{Dir: t.TempDir()})
	result, err := service.Sync(context.Background(), SyncOptions{})
	if err != nil {
		t.Fatalf("同步工具失败: %v", err)
	}
	if _, ok := result.Find("go", otherOS, "arm64"); !ok {
		t.Fatalf("其他平台锁项丢失: %#v", result.Tools)
	}
}

func TestServiceVerify_whenBuildChanged_shouldRejectLockDrift(t *testing.T) {
	root, document := writeToolProject(t)
	service := newTestService(root, document, projecttrust.Store{Dir: t.TempDir()})
	if _, err := service.Sync(context.Background(), SyncOptions{}); err != nil {
		t.Fatalf("准备锁文件失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, buildspec.FileName), []byte("version = 1\n# changed\n"), 0o644); err != nil {
		t.Fatalf("修改描述文件失败: %v", err)
	}
	if _, err := service.Verify(context.Background()); err == nil || !strings.Contains(err.Error(), "摘要不一致") {
		t.Fatalf("错误 = %v", err)
	}
}

func writeToolProject(t *testing.T) (string, buildspec.Document) {
	t.Helper()
	root := t.TempDir()
	content := "version = 1\n[tools.go]\ntype = \"system\"\ncommand = \"go\"\ninstall = \"manual\"\n"
	path := filepath.Join(root, buildspec.FileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入描述文件失败: %v", err)
	}
	document, err := buildspec.LoadFile(path)
	if err != nil {
		t.Fatalf("加载描述文件失败: %v", err)
	}
	return root, document
}

func newTestService(root string, document buildspec.Document, trust projecttrust.Store) Service {
	return Service{
		Root: root, Document: document, Environment: environmentMap(os.Environ()),
		Manager: tooling.NewManager(root, tCacheDir(root), environmentMap(os.Environ())), Trust: trust,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}
}

func tCacheDir(root string) string {
	return filepath.Join(root, "tool-cache")
}

func environmentMap(values []string) map[string]string {
	result := make(map[string]string)
	for _, value := range values {
		name, item, ok := strings.Cut(value, "=")
		if ok {
			result[name] = item
		}
	}
	return result
}

func writeGoToolProject(t *testing.T, install string) (string, buildspec.Document) {
	t.Helper()
	root := t.TempDir()
	content := "version = 1\n[tools.demo]\ntype = \"go\"\npackage = \"example.com/tools/cmd/demo\"\nversion = \"v1.0.0\"\ninstall = \"" + install + "\"\n"
	path := filepath.Join(root, buildspec.FileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入描述文件失败: %v", err)
	}
	document, err := buildspec.LoadFile(path)
	if err != nil {
		t.Fatalf("加载描述文件失败: %v", err)
	}
	return root, document
}

func fakeGoToolService(t *testing.T, root string, document buildspec.Document, installCount *int) Service {
	t.Helper()
	cache := filepath.Join(root, "tool-cache")
	manager := tooling.NewManager(root, cache, environmentMap(os.Environ()))
	manager.InstallGo = func(_ context.Context, _ string, _ string, destination string, _ map[string]string) error {
		*installCount++
		path := filepath.Join(destination, executableNameForTest("demo"))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("tool\n"), 0o755)
	}
	manager.ReadBuild = func(string) (tooling.BuildMetadata, error) {
		return tooling.BuildMetadata{Module: "example.com/tools", Version: "v1.0.0", Sum: "h1:sum"}, nil
	}
	return Service{
		Root: root, Document: document, Environment: environmentMap(os.Environ()), Manager: manager,
		Trust: projecttrust.Store{Dir: t.TempDir()}, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}
}

func executableNameForTest(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
