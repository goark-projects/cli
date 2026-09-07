package clitest

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_whenProjectDiscoveryFails_shouldPreserveExitSemantics(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "canceled", err: context.Canceled, want: 130},
		{name: "Go process exit", err: processExitError{code: 23}, want: 23},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeTestModule(t, map[string]string{
				"go.mod": "module example.com/failure\n\ngo 1.27.0\n",
			})
			command := Command{
				Dir:    root,
				Out:    io.Discard,
				Err:    io.Discard,
				Runner: &recordingProcessRunner{err: test.err},
			}
			if code := command.Run([]string{"build", "./..."}); code != test.want {
				t.Fatalf("退出码 = %d, want %d", code, test.want)
			}
		})
	}
}

func TestCommand_whenGenerateRequested_shouldGenerateAnnotatedPackages(t *testing.T) {
	root := annotatedTestProject(t)
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"generate"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	output := filepath.Join(
		canonicalTestPath(t, root),
		"internal",
		"app",
		"gen",
		"zz_goark_core_gen.go",
	)
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("生成文件不存在: %v", err)
	}
	if !strings.Contains(stderr.String(), "generated "+output) {
		t.Fatalf("生成诊断缺失: %q", stderr.String())
	}
}

func TestCommand_whenGenerateUsesBuildTags_shouldGenerateSelectedFileSet(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.26.0\n",
		"app/base.go": "package app\n",
		"app/tagged.go": `//go:build special

package app

//goark:component
type TaggedComponent struct{}
`,
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"generate", "-tags", "special", "./app"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "app", "gen", "zz_goark_core_gen.go"))
	if err != nil {
		t.Fatalf("读取生成文件失败: %v", err)
	}
	if !strings.Contains(string(data), "taggedComponent") {
		t.Fatalf("构建标签生成内容错误:\n%s", data)
	}
}

func TestCommand_whenGenerateUsesDirectoryFlag_shouldResolveProjectFromThatDirectory(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "service")
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatalf("创建项目目录失败: %v", err)
	}
	goMod := []byte("module example.com/service\n\ngo 1.26.0\n")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("写入 go.mod 失败: %v", err)
	}
	buildFile := filepath.Join(root, "goark.build")
	if err := os.WriteFile(buildFile, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("写入 goark.build 失败: %v", err)
	}
	source := []byte("package app\n\n//goark:component\ntype App struct{}\n")
	if err := os.WriteFile(filepath.Join(root, "app", "app.go"), source, 0o644); err != nil {
		t.Fatalf("写入源码失败: %v", err)
	}
	var stderr bytes.Buffer
	command := testOSCommand(parent, io.Discard, &stderr)

	if code := command.Run([]string{"generate", "-C", "service"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "app", "gen", "zz_goark_core_gen.go")); err != nil {
		t.Fatalf("生成文件不存在: %v", err)
	}
}

func TestCommand_whenRemovedRunGenerateOnlyRequested_shouldReject(t *testing.T) {
	root := annotatedTestProject(t)
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"run", "--goark-generate-only"}); code != 2 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "已删除参数") {
		t.Fatalf("错误缺失: %q", stderr.String())
	}
}

func TestCommand_whenBuildDryRunRequested_shouldPrintPlanWithoutWriting(t *testing.T) {
	root := annotatedTestProject(t)
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"build", "--goark-dry-run", "./..."}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(
		filepath.Join(root, "internal", "app", "gen", "zz_goark_core_gen.go"),
	); !os.IsNotExist(err) {
		t.Fatalf("模拟执行不应写文件: %v", err)
	}
	diagnostic := stderr.String()
	if !strings.Contains(diagnostic, "would generate") ||
		!strings.Contains(diagnostic, "go build ./...") {
		t.Fatalf("执行计划不完整: %q", stderr.String())
	}
}

func TestCommand_whenBuildDryRunRequested_shouldNotStartAnyProcess(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.26.0\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
	})
	var stderr bytes.Buffer
	runner := &recordingProcessRunner{}
	command := Command{Dir: root, Out: io.Discard, Err: &stderr, Runner: runner}

	if code := command.Run([]string{"build", "--goark-dry-run", "./..."}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if len(runner.requests) != 0 {
		t.Fatalf("模拟执行不应启动任何进程: %#v", runner.requests)
	}
}

func TestCommand_whenBuildOutputConfigured_shouldPassOutputToGoBuild(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.26.0\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
		"goark.build":        "version = 1\n[commands.build]\noutput = \"./build/app\"\n",
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"build", "--goark-dry-run", "./cmd/server"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "go build -o ./build/app ./cmd/server") {
		t.Fatalf("构建输出参数缺失: %q", stderr.String())
	}
}

func TestCommand_whenBuildTargetOmitted_shouldUseConfiguredProjectMain(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.26.0\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
		"goark.build": `version = 1
[project]
main = "./cmd/server"
[commands.build]
go-args = ["-trimpath"]
output = "./build/app"
`,
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"build", "--goark-dry-run"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "go build -o ./build/app -trimpath ./cmd/server") {
		t.Fatalf("构建计划未使用 project.main: %q", stderr.String())
	}
}

func TestCommand_whenBuildOutputProvidedByCLI_shouldOverrideConfiguredOutput(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.26.0\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
		"goark.build":        "version = 1\n[commands.build]\noutput = \"./build/configured\"\n",
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	args := []string{
		"build", "--goark-dry-run", "-o", "./build/cli", "./cmd/server",
	}
	if code := command.Run(args); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	diagnostic := stderr.String()
	if !strings.Contains(diagnostic, "go build -o ./build/cli ./cmd/server") ||
		strings.Contains(diagnostic, "./build/configured") {
		t.Fatalf("CLI 输出未覆盖配置输出: %q", stderr.String())
	}
}

func TestCommand_whenRunDryRunContainsSecretArguments_shouldRedactDiagnostic(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.26.0\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
	})
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: io.Discard, Err: &stderr, Runner: &recordingProcessRunner{}}

	if code := command.Run([]string{
		"run", "--goark-dry-run", "--goark-env=API_TOKEN=environment-secret",
		"./cmd/server", "--token=argument-secret", "environment-secret", "value with spaces",
	}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	diagnostic := stderr.String()
	if strings.Contains(diagnostic, "argument-secret") ||
		strings.Contains(diagnostic, "environment-secret") {
		t.Fatalf("模拟执行泄露密钥: %q", diagnostic)
	}
	if !strings.Contains(diagnostic, "--token=******") ||
		!strings.Contains(diagnostic, `"value with spaces"`) {
		t.Fatalf("模拟执行参数格式错误: %q", diagnostic)
	}
}

func TestCommand_whenLockedBuildHasNoLockFile_shouldRejectEvenWithoutTools(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.26.0\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)
	args := []string{"build", "--goark-locked", "--goark-dry-run", "./..."}
	if code := command.Run(args); code == 0 {
		t.Fatalf("锁文件缺失时 --goark-locked 必须失败: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "goark.build.lock") {
		t.Fatalf("错误未说明锁文件缺失: %s", stderr.String())
	}
}

func TestCommand_whenBuildLifecycleConfigured_shouldPlanHooksInFixedOrder(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.26.0\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
		"goark.build": `version = 1
[project]
main = "./cmd/server"

[commands.generate]
before = ["generate-before"]
after = ["generate-after"]

[commands.build]
before = ["build-before"]
after = ["build-after"]
finally = ["build-finally"]

[tasks.generate-before]
type = "go"
args = ["version"]

[tasks.generate-after]
type = "go"
args = ["version"]

[tasks.build-before]
type = "go"
args = ["version"]

[tasks.build-after]
type = "go"
args = ["version"]

[tasks.build-finally]
type = "go"
args = ["version"]
`,
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)
	if code := command.Run([]string{"build", "--goark-dry-run", "./..."}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	assertOrderedFragments(t, stderr.String(), []string{
		"would run task generate-before",
		"would run task generate-after",
		"would run task build-before",
		"would run: go build ./...",
		"would run task build-after",
		"would run task build-finally",
	})
}

func assertOrderedFragments(t *testing.T, value string, fragments []string) {
	t.Helper()
	position := 0
	for _, fragment := range fragments {
		index := strings.Index(value[position:], fragment)
		if index < 0 {
			t.Fatalf("输出中缺少有序片段 %q:\n%s", fragment, value)
		}
		position += index + len(fragment)
	}
}
