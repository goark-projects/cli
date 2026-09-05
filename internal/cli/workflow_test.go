package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_whenGenerateRequested_shouldGenerateAnnotatedPackages(t *testing.T) {
	root := annotatedTestProject(t)
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"generate"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	output := filepath.Join(root, "internal", "app", "zz_goark_app_gen.go")
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("生成文件不存在: %v", err)
	}
	if !strings.Contains(stderr.String(), "generated "+output) {
		t.Fatalf("生成诊断缺失: %q", stderr.String())
	}
}

func TestCommand_whenGenerateUsesBuildTags_shouldGenerateSelectedFileSet(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
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
	data, err := os.ReadFile(filepath.Join(root, "app", "zz_goark_app_gen.go"))
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
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/service\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("写入 go.mod 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "goark.build"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("写入 goark.build 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "app.go"), []byte("package app\n\n//goark:component\ntype App struct{}\n"), 0o644); err != nil {
		t.Fatalf("写入源码失败: %v", err)
	}
	var stderr bytes.Buffer
	command := testOSCommand(parent, io.Discard, &stderr)

	if code := command.Run([]string{"generate", "-C", "service"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "app", "zz_goark_app_gen.go")); err != nil {
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
	if _, err := os.Stat(filepath.Join(root, "internal", "app", "zz_goark_app_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("模拟执行不应写文件: %v", err)
	}
	if !strings.Contains(stderr.String(), "would generate") || !strings.Contains(stderr.String(), "go build ./...") {
		t.Fatalf("执行计划不完整: %q", stderr.String())
	}
}

func TestCommand_whenBuildDryRunRequested_shouldNotStartAnyProcess(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
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
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
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

func TestCommand_whenBuildOutputProvidedByCLI_shouldOverrideConfiguredOutput(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
		"goark.build":        "version = 1\n[commands.build]\noutput = \"./build/configured\"\n",
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"build", "--goark-dry-run", "-o", "./build/cli", "./cmd/server"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "go build -o ./build/cli ./cmd/server") || strings.Contains(stderr.String(), "./build/configured") {
		t.Fatalf("CLI 输出未覆盖配置输出: %q", stderr.String())
	}
}

func TestCommand_whenRunDryRunContainsSecretArguments_shouldRedactDiagnostic(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
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
	if strings.Contains(diagnostic, "argument-secret") || strings.Contains(diagnostic, "environment-secret") {
		t.Fatalf("模拟执行泄露密钥: %q", diagnostic)
	}
	if !strings.Contains(diagnostic, "--token=******") || !strings.Contains(diagnostic, `"value with spaces"`) {
		t.Fatalf("模拟执行参数格式错误: %q", diagnostic)
	}
}

func TestCommand_whenBuildLifecycleConfigured_shouldPlanHooksInFixedOrder(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
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

func TestCommand_whenGoCommandFails_shouldStillRunFinally(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.25\n",
		"goark.build": `version = 1
[commands.build]
finally = ["cleanup"]

[tasks.cleanup]
type = "go"
args = ["version"]
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := testOSCommand(root, &stdout, &stderr)
	if code := command.Run([]string{"build", "./missing"}); code == 0 {
		t.Fatalf("构建必须失败, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "go version") {
		t.Fatalf("finally 未执行: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
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

func TestCommand_whenInfoRequested_shouldReportProjectAndGenerationPlan(t *testing.T) {
	root := annotatedTestProject(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := &recordingProcessRunner{}
	command := Command{Dir: root, Out: &stdout, Err: &stderr, Runner: runner, TrustDir: t.TempDir(), ToolCacheDir: t.TempDir()}

	if code := command.Run([]string{"info"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	for _, fragment := range []string{"Goark CLI:", "Project: app", "Module: example.com/app", "Main: ./cmd/server", "Profile: (none)", "Generators: annotations", "Generated packages: 1", "Execution plans:"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("info 缺少 %q:\n%s", fragment, stdout.String())
		}
	}
	if len(runner.requests) != 0 {
		t.Fatalf("info 不应启动任何进程: %#v", runner.requests)
	}
}

func TestCommand_whenInfoJSONRequested_shouldReportMachineReadableDiagnostics(t *testing.T) {
	root := annotatedTestProject(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := &recordingProcessRunner{}
	command := Command{Dir: root, Out: &stdout, Err: &stderr, Runner: runner, TrustDir: t.TempDir(), ToolCacheDir: t.TempDir()}

	if code := command.Run([]string{"info", "--json"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	var info struct {
		CLIVersion string `json:"cliVersion"`
		Project    struct {
			Module string `json:"module"`
			Main   string `json:"main"`
		} `json:"project"`
		Profile    string `json:"profile"`
		Generators []struct {
			Patterns []string `json:"patterns"`
			Packages int      `json:"packages"`
		} `json:"generators"`
		Plans []struct {
			Command string `json:"command"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("JSON 无效: %v\n%s", err, stdout.String())
	}
	if info.CLIVersion != Version || info.Project.Module != "example.com/app" || info.Project.Main != "./cmd/server" || info.Profile != "" {
		t.Fatalf("诊断信息错误: %#v", info)
	}
	if len(info.Generators) != 1 || info.Generators[0].Packages != 1 || len(info.Generators[0].Patterns) != 1 || info.Generators[0].Patterns[0] != "./..." {
		t.Fatalf("生成信息错误: %#v", info.Generators)
	}
	if len(info.Plans) != 8 || info.Plans[0].Command != "build" {
		t.Fatalf("执行计划错误: %#v", info.Plans)
	}
	if len(runner.requests) != 0 {
		t.Fatalf("info --json 不应启动任何进程: %#v", runner.requests)
	}
}

func TestCommand_whenInfoContainsSecretEnvironment_shouldRedactWithoutSideEffects(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"goark.build": "version = 1\n[commands.build.environment]\nAPI_TOKEN = \"top-secret-value\"\n",
	})
	runner := &recordingProcessRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := Command{
		Dir: root, Out: &stdout, Err: &stderr, Runner: runner,
		Env: []string{"UNDECLARED_PROCESS_VALUE=must-not-appear"}, TrustDir: t.TempDir(), ToolCacheDir: t.TempDir(),
	}
	if code := command.Run([]string{"info", "--json"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "top-secret-value") || !strings.Contains(stdout.String(), `"API_TOKEN":"******"`) {
		t.Fatalf("info 密钥脱敏错误: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "UNDECLARED_PROCESS_VALUE") || strings.Contains(stdout.String(), "must-not-appear") {
		t.Fatalf("info 不应输出未声明的进程环境: %s", stdout.String())
	}
	if len(runner.requests) != 0 {
		t.Fatalf("info 不应启动进程: %#v", runner.requests)
	}
	if _, err := os.Stat(filepath.Join(root, ".goark")); !os.IsNotExist(err) {
		t.Fatalf("info 不应创建项目状态目录: %v", err)
	}
}

func TestCommand_whenRunRequested_shouldPassPropertiesAndApplicationArguments(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.25\n",
		"cmd/server/main.go": `package main

import (
	"fmt"
	"os"
)

func main() {
	for _, argument := range os.Args[1:] {
		fmt.Println(argument)
	}
}
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := testOSCommand(root, &stdout, &stderr)

	code := command.Run([]string{
		"run",
		"-Dserver.port=9090",
		"./cmd/server",
		"--feature.enabled=true",
		"--",
		"--job=sync",
		"input.json",
	})
	if code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	want := "-Dserver.port=9090\n--feature.enabled=true\n--job=sync\ninput.json\n"
	if stdout.String() != want {
		t.Fatalf("应用参数输出 = %q, want %q", stdout.String(), want)
	}
}

func annotatedTestProject(t *testing.T) string {
	return writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
		"internal/app/wiring.go": `package app

//goark:service
type UserService struct{}
`,
	})
}

func testOSCommand(dir string, stdout io.Writer, stderr io.Writer) Command {
	return Command{
		Dir:    dir,
		Env:    append(os.Environ(), "GOWORK=off", "GOFLAGS="),
		Out:    stdout,
		Err:    stderr,
		Runner: osProcessRunner{},
	}
}
