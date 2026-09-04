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

func TestCommand_whenRunGenerateOnlyRequested_shouldResolveMainAndNotStartApplication(t *testing.T) {
	root := annotatedTestProject(t)
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)

	if code := command.Run([]string{"run", "--goark-generate-only"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "app", "zz_goark_app_gen.go")); err != nil {
		t.Fatalf("生成文件不存在: %v", err)
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

func TestCommand_whenInfoRequested_shouldReportProjectAndGenerationPlan(t *testing.T) {
	root := annotatedTestProject(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := testOSCommand(root, &stdout, &stderr)

	if code := command.Run([]string{"info"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	for _, fragment := range []string{"Goark CLI:", "Go toolchain:", "Module: example.com/app", "Main: ./cmd/server", "Generators: annotations", "Generated packages: 1"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("info 缺少 %q:\n%s", fragment, stdout.String())
		}
	}
}

func TestCommand_whenInfoJSONRequested_shouldReportMachineReadableDiagnostics(t *testing.T) {
	root := annotatedTestProject(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := testOSCommand(root, &stdout, &stderr)

	if code := command.Run([]string{"info", "--json"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	var info struct {
		CLIVersion         string   `json:"cliVersion"`
		Module             string   `json:"module"`
		Main               string   `json:"main"`
		GenerationPatterns []string `json:"generationPatterns"`
		GeneratedPackages  int      `json:"generatedPackages"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("JSON 无效: %v\n%s", err, stdout.String())
	}
	if info.CLIVersion != Version || info.Module != "example.com/app" || info.Main != "./cmd/server" || info.GeneratedPackages != 1 {
		t.Fatalf("诊断信息错误: %#v", info)
	}
	if len(info.GenerationPatterns) != 1 || info.GenerationPatterns[0] != "./..." {
		t.Fatalf("生成范围错误: %#v", info.GenerationPatterns)
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
		"--goark-no-generate",
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
