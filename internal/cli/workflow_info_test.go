package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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
			Command              string   `json:"command"`
			GoArguments          []string `json:"goArguments"`
			ApplicationArguments []string `json:"applicationArguments"`
			Before               []string `json:"before"`
			After                []string `json:"after"`
			Finally              []string `json:"finally"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("JSON 无效: %v\n%s", err, stdout.String())
	}
	if info.CLIVersion != "devel" || info.Project.Module != "example.com/app" || info.Project.Main != "./cmd/server" || info.Profile != "" {
		t.Fatalf("诊断信息错误: %#v", info)
	}
	if len(info.Generators) != 1 || info.Generators[0].Packages != 1 || len(info.Generators[0].Patterns) != 1 || info.Generators[0].Patterns[0] != "./..." {
		t.Fatalf("生成信息错误: %#v", info.Generators)
	}
	if len(info.Plans) != 8 || info.Plans[0].Command != "build" {
		t.Fatalf("执行计划错误: %#v", info.Plans)
	}
	for _, plan := range info.Plans {
		if plan.GoArguments == nil || plan.ApplicationArguments == nil || plan.Before == nil || plan.After == nil || plan.Finally == nil {
			t.Fatalf("执行计划数组必须稳定输出为空数组而不是 null: %#v", plan)
		}
		if plan.Command == "run" && !reflect.DeepEqual(plan.GoArguments, []string{"run", "./cmd/server"}) {
			t.Fatalf("run 最终执行计划错误: %#v", plan.GoArguments)
		}
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

func TestCommand_whenInfoProfileSelected_shouldApplyProfileToPlans(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
		"goark.build": `version = 1
[project]
main = "./cmd/server"
[profiles.dev]
go-args = ["-tags=dev"]
[profiles.dev.environment]
API_TOKEN = "profile-secret"
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := Command{Dir: root, Out: &stdout, Err: &stderr, Runner: &recordingProcessRunner{}, TrustDir: t.TempDir(), ToolCacheDir: t.TempDir()}

	if code := command.Run([]string{"info", "--goark-profile=dev", "--json"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	var report infoReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("JSON 无效: %v\n%s", err, stdout.String())
	}
	if report.Profile != "dev" {
		t.Fatalf("Profile = %q", report.Profile)
	}
	for _, plan := range report.Plans {
		if !containsAll(strings.Join(plan.GoArguments, " "), "-tags=dev") {
			t.Fatalf("计划未合并 Profile 参数: %#v", plan)
		}
		if plan.Environment["API_TOKEN"] != "******" {
			t.Fatalf("计划未合并或脱敏 Profile 环境: %#v", plan.Environment)
		}
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
