package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/tooling"
)

func TestResolveVerifiedLifecycleTool_whenTrustedToolDigestDrifts_shouldRestoreAndVerify(t *testing.T) {
	cache := t.TempDir()
	manager := tooling.NewManager(t.TempDir(), cache, nil)
	installCount := 0
	manager.InstallGo = func(_ context.Context, _ string, _ string, destination string, _ map[string]string) error {
		installCount++
		path := filepath.Join(destination, lifecycleToolExecutableName("demo"))
		if err := os.MkdirAll(destination, 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("locked-tool\n"), 0o755)
	}
	manager.ReadBuild = func(path string) (tooling.BuildMetadata, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return tooling.BuildMetadata{}, err
		}
		if string(data) != "locked-tool\n" {
			return tooling.BuildMetadata{}, fmt.Errorf("invalid Go build info")
		}
		return tooling.BuildMetadata{Module: "example.com/tools", Version: "v1.0.0", Sum: "h1:sum"}, nil
	}
	tool := buildspec.Tool{Type: buildspec.ToolTypeGo, Package: "example.com/tools/cmd/demo", Version: "v1.0.0", Install: "auto"}
	locked, err := manager.Resolve(context.Background(), "demo", tool, tooling.ResolveOptions{AllowInstall: true})
	if err != nil {
		t.Fatalf("准备锁定工具失败: %v", err)
	}
	if err := os.WriteFile(locked.Path, []byte("corrupted\n"), 0o755); err != nil {
		t.Fatalf("模拟工具损坏失败: %v", err)
	}

	restored, err := resolveVerifiedLifecycleTool(context.Background(), manager, "demo", tool, locked.Entry, true, false)
	if err != nil {
		t.Fatalf("可信工具恢复失败: %v", err)
	}
	if installCount != 2 || restored.Entry.SHA256 != locked.Entry.SHA256 {
		t.Fatalf("恢复结果错误: count=%d restored=%#v locked=%#v", installCount, restored.Entry, locked.Entry)
	}
}

func TestResolveVerifiedLifecycleTool_whenUntrustedToolDigestDrifts_shouldRejectWithoutRestore(t *testing.T) {
	cache := t.TempDir()
	manager := tooling.NewManager(t.TempDir(), cache, nil)
	installCount := 0
	manager.InstallGo = func(_ context.Context, _ string, _ string, destination string, _ map[string]string) error {
		installCount++
		if err := os.MkdirAll(destination, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destination, lifecycleToolExecutableName("demo")), []byte("locked-tool\n"), 0o755)
	}
	manager.ReadBuild = func(string) (tooling.BuildMetadata, error) {
		return tooling.BuildMetadata{Module: "example.com/tools", Version: "v1.0.0", Sum: "h1:sum"}, nil
	}
	tool := buildspec.Tool{Type: buildspec.ToolTypeGo, Package: "example.com/tools/cmd/demo", Version: "v1.0.0", Install: "auto"}
	locked, err := manager.Resolve(context.Background(), "demo", tool, tooling.ResolveOptions{AllowInstall: true})
	if err != nil {
		t.Fatalf("准备锁定工具失败: %v", err)
	}
	if err := os.WriteFile(locked.Path, []byte("corrupted\n"), 0o755); err != nil {
		t.Fatalf("模拟工具损坏失败: %v", err)
	}

	if _, err := resolveVerifiedLifecycleTool(context.Background(), manager, "demo", tool, locked.Entry, false, false); err == nil {
		t.Fatal("未信任工具损坏后必须失败")
	}
	if installCount != 1 {
		t.Fatalf("未信任工具被自动重装: %d", installCount)
	}
}

func TestValidateTaskPaths_whenWorkingDirectoryContainsVariable_shouldDeferValidation(t *testing.T) {
	project := goarkProject{
		Root: t.TempDir(),
		Build: buildspec.Document{Tasks: map[string]buildspec.Task{
			"generate": {WorkingDirectory: "${env:WORKDIR}"},
		}},
	}
	if err := validateTaskPaths(project); err != nil {
		t.Fatalf("变量工作目录应延后到展开后校验: %v", err)
	}
}

func TestPrepareLifecycle_whenCapturingGoVersion_shouldUseProjectRootAndPlanEnvironment(t *testing.T) {
	root := t.TempDir()
	runner := &recordingProcessRunner{}
	plan := buildplan.Plan{Environment: map[string]string{"GOTOOLCHAIN": "local", "GOENV": "off"}}
	project := goarkProject{
		Root: root,
		Build: buildspec.Document{
			Execution: buildspec.Execution{DefaultTimeout: buildspec.Duration{}},
			Tasks:     map[string]buildspec.Task{},
		},
	}
	command := Command{Context: context.Background(), Dir: t.TempDir(), Runner: runner}

	if _, err := command.prepareLifecycleTargets(project, plan, nil); err != nil {
		t.Fatalf("准备生命周期失败: %v", err)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("Go 版本探测请求数 = %d", len(runner.requests))
	}
	request := runner.requests[0]
	if request.Dir != root {
		t.Fatalf("Go 版本探测目录 = %q, want %q", request.Dir, root)
	}
	if !containsEnvironmentEntry(request.Env, "GOTOOLCHAIN=local") || !containsEnvironmentEntry(request.Env, "GOENV=off") {
		t.Fatalf("Go 版本探测环境未使用最终计划: %#v", request.Env)
	}
}

func containsEnvironmentEntry(environment []string, want string) bool {
	for _, entry := range environment {
		if entry == want {
			return true
		}
	}
	return false
}

func lifecycleToolExecutableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
