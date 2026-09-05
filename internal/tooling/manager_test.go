package tooling

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
)

func TestManagerResolve_whenSystemToolExists_shouldCreateVerifiableLockEntry(t *testing.T) {
	root := t.TempDir()
	executable := writeExecutable(t, root, "system-tool")
	manager := NewManager(root, t.TempDir(), map[string]string{})
	manager.LookPath = func(command string, _ map[string]string) (string, error) {
		return executable, nil
	}

	resolved, err := manager.Resolve(context.Background(), "sha256", buildspec.Tool{Type: buildspec.ToolTypeSystem, Command: "sha256sum"}, ResolveOptions{})
	if err != nil {
		t.Fatalf("解析系统工具失败: %v", err)
	}
	if resolved.Path != executable || resolved.Entry.Name != "sha256" || resolved.Entry.Type != buildspec.ToolTypeSystem {
		t.Fatalf("解析结果 = %#v", resolved)
	}
	if err := Verify(resolved, resolved.Entry); err != nil {
		t.Fatalf("验证锁定项失败: %v", err)
	}

	changed := resolved.Entry
	changed.SHA256 = strings.Repeat("0", 64)
	if err := Verify(resolved, changed); err == nil || !strings.Contains(err.Error(), "摘要") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestManagerResolve_whenPathComesFromMergedEnvironment_shouldFindSystemTool(t *testing.T) {
	directory := t.TempDir()
	writeExecutable(t, directory, executableName("custom-tool"))
	manager := NewManager(t.TempDir(), t.TempDir(), map[string]string{"PATH": directory})

	resolved, err := manager.Resolve(context.Background(), "custom", buildspec.Tool{Type: buildspec.ToolTypeSystem, Command: "custom-tool"}, ResolveOptions{})
	if err != nil {
		t.Fatalf("从合并环境解析工具失败: %v", err)
	}
	if filepath.Dir(resolved.Path) != directory {
		t.Fatalf("工具路径 = %q", resolved.Path)
	}
}

func TestManagerResolve_whenLocalToolEscapesProject_shouldReject(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(root, t.TempDir(), nil)
	_, err := manager.Resolve(context.Background(), "local", buildspec.Tool{Type: buildspec.ToolTypeLocal, Path: "../outside"}, ResolveOptions{})
	if err == nil || !strings.Contains(err.Error(), "项目根目录") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestManagerResolve_whenGoToolMissingAndOffline_shouldReject(t *testing.T) {
	manager := NewManager(t.TempDir(), t.TempDir(), nil)
	_, err := manager.Resolve(context.Background(), "demo", buildspec.Tool{
		Type: buildspec.ToolTypeGo, Package: "example.com/tools/demo", Version: "v1.0.0", Install: "auto",
	}, ResolveOptions{AllowInstall: true, Offline: true})
	if err == nil || !strings.Contains(err.Error(), "离线") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestManagerResolve_whenGoToolInstallationAllowed_shouldInstallOnce(t *testing.T) {
	cache := t.TempDir()
	manager := NewManager(t.TempDir(), cache, nil)
	installCount := 0
	manager.InstallGo = func(_ context.Context, _ string, _ string, destination string, _ map[string]string) error {
		installCount++
		writeExecutable(t, destination, executableName("demo"))
		return nil
	}
	manager.ReadBuild = func(string) (BuildMetadata, error) {
		return BuildMetadata{Module: "example.com/tools", Version: "v1.0.0", Sum: "h1:module-sum"}, nil
	}
	spec := buildspec.Tool{Type: buildspec.ToolTypeGo, Package: "example.com/tools/cmd/demo", Version: "v1.0.0", Install: "auto"}

	first, err := manager.Resolve(context.Background(), "demo", spec, ResolveOptions{AllowInstall: true})
	if err != nil {
		t.Fatalf("安装 Go 工具失败: %v", err)
	}
	second, err := manager.Resolve(context.Background(), "demo", spec, ResolveOptions{})
	if err != nil {
		t.Fatalf("复用 Go 工具失败: %v", err)
	}
	if installCount != 1 || first.Path != second.Path {
		t.Fatalf("安装次数 = %d, first=%q second=%q", installCount, first.Path, second.Path)
	}
	if first.Entry.Module != "example.com/tools" || first.Entry.ModuleSum != "h1:module-sum" {
		t.Fatalf("构建元数据 = %#v", first.Entry)
	}
}

func TestManagerResolve_whenGoToolExecutableMissingFromExistingCache_shouldRestore(t *testing.T) {
	cache := t.TempDir()
	manager := NewManager(t.TempDir(), cache, nil)
	installCount := 0
	manager.InstallGo = func(_ context.Context, _ string, _ string, destination string, _ map[string]string) error {
		installCount++
		writeExecutable(t, destination, executableName("demo"))
		return nil
	}
	manager.ReadBuild = func(string) (BuildMetadata, error) {
		return BuildMetadata{Module: "example.com/tools", Version: "v1.0.0", Sum: "h1:module-sum"}, nil
	}
	spec := buildspec.Tool{Type: buildspec.ToolTypeGo, Package: "example.com/tools/cmd/demo", Version: "v1.0.0", Install: "auto"}

	first, err := manager.Resolve(context.Background(), "demo", spec, ResolveOptions{AllowInstall: true})
	if err != nil {
		t.Fatalf("首次安装失败: %v", err)
	}
	if err := os.Remove(first.Path); err != nil {
		t.Fatalf("模拟工具文件缺失失败: %v", err)
	}
	second, err := manager.Resolve(context.Background(), "demo", spec, ResolveOptions{AllowInstall: true})
	if err != nil {
		t.Fatalf("恢复工具失败: %v", err)
	}
	if installCount != 2 || first.Path != second.Path {
		t.Fatalf("安装次数 = %d, first=%q second=%q", installCount, first.Path, second.Path)
	}
}

func TestManagerResolve_whenForceInstallRequested_shouldReplaceExistingTool(t *testing.T) {
	cache := t.TempDir()
	manager := NewManager(t.TempDir(), cache, nil)
	installCount := 0
	manager.InstallGo = func(_ context.Context, _ string, _ string, destination string, _ map[string]string) error {
		installCount++
		path := writeExecutable(t, destination, executableName("demo"))
		return os.WriteFile(path, []byte(fmt.Sprintf("tool-%d\n", installCount)), 0o755)
	}
	manager.ReadBuild = func(string) (BuildMetadata, error) {
		return BuildMetadata{Module: "example.com/tools", Version: "v1.0.0", Sum: "h1:module-sum"}, nil
	}
	spec := buildspec.Tool{Type: buildspec.ToolTypeGo, Package: "example.com/tools/cmd/demo", Version: "v1.0.0", Install: "auto"}

	first, err := manager.Resolve(context.Background(), "demo", spec, ResolveOptions{AllowInstall: true})
	if err != nil {
		t.Fatalf("首次安装失败: %v", err)
	}
	second, err := manager.Resolve(context.Background(), "demo", spec, ResolveOptions{AllowInstall: true, ForceInstall: true})
	if err != nil {
		t.Fatalf("强制重装失败: %v", err)
	}
	data, err := os.ReadFile(second.Path)
	if err != nil {
		t.Fatalf("读取重装工具失败: %v", err)
	}
	if installCount != 2 || first.Entry.SHA256 == second.Entry.SHA256 || string(data) != "tool-2\n" {
		t.Fatalf("强制重装结果错误: count=%d first=%#v second=%#v data=%q", installCount, first.Entry, second.Entry, data)
	}
}

func writeExecutable(t *testing.T, root string, name string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建工具目录失败: %v", err)
	}
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("test executable\n"), 0o755); err != nil {
		t.Fatalf("写入测试工具失败: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatalf("设置执行权限失败: %v", err)
		}
	}
	return path
}
