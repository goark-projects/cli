package clitest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerationUsesConfiguredGOOS(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":                   "module example.com/platform\n\ngo 1.27.0\n",
		"goark.build":              "version = 1\n[commands.generate.environment]\nGOOS = \"linux\"\n",
		"app/base.go":              "package app\n",
		"app/component_linux.go":   "package app\n//goark:component\ntype LinuxService struct{}\n",
		"app/component_windows.go": "package app\n//goark:unknown\ntype WindowsService struct{}\n",
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)
	command.Env = append(command.Env, "GOOS=windows")
	if code := command.Run([]string{"generate"}); code != 0 {
		t.Fatalf("生成失败: %d, %s", code, &stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, "app", "gen", "zz_goark_core_gen.go"))
	if err != nil || !bytes.Contains(data, []byte("LinuxService")) {
		t.Fatalf("未按最终 GOOS 生成: %v\n%s", err, data)
	}
}

func TestWorkflowPreservesPersistedReadonlyMode(t *testing.T) {
	for _, action := range []string{"generate", "build", "run", "test"} {
		t.Run(action, func(t *testing.T) {
			mod := "module example.com/readonly\n\ngo 1.27.0\n\n" +
				"replace example.com/dependency => ./dependency\n"
			root := writeTestModule(t, map[string]string{
				"go.mod": mod,
				"main.go": "package main\nimport _ \"example.com/dependency\"\n" +
					"func main() {}\n",
				"dependency/go.mod": "module example.com/dependency\n\ngo 1.27.0\n",
				"dependency/dep.go": "package dependency\n",
				"goenv":             "GOFLAGS=-mod=readonly\n",
			})
			var stderr bytes.Buffer
			command := testOSCommand(root, io.Discard, &stderr)
			command.Env = append(command.Env,
				"GOENV="+filepath.Join(root, "goenv"), "GOPROXY=off", "GOTOOLCHAIN=local",
			)
			code := command.Run([]string{action})
			data, err := os.ReadFile(filepath.Join(root, "go.mod"))
			if err != nil || string(data) != mod {
				t.Fatalf("只读模式修改了 go.mod: %v\n%s\n%s", err, data, &stderr)
			}
			if _, err := os.Stat(filepath.Join(root, "go.sum")); !os.IsNotExist(err) {
				t.Fatalf("只读模式创建了 go.sum: %v", err)
			}
			if action != "generate" && (code == 0 ||
				!strings.Contains(stderr.String(), "example.com/dependency")) {
				t.Fatalf("未保留 Go 只读诊断: code=%d, %s", code, &stderr)
			}
		})
	}
}

func TestWorkflowAllowsExplicitReadonlyOverride(t *testing.T) {
	for _, mode := range []string{"命令行", "进程环境", "关闭持久化配置"} {
		t.Run(mode, func(t *testing.T) {
			root := writeTestModule(t, map[string]string{
				"go.mod": "module example.com/override\n\ngo 1.27.0\n\n" +
					"replace example.com/dependency => ./dependency\n",
				"main.go": "package main\nimport _ \"example.com/dependency\"\n" +
					"func main() {}\n",
				"dependency/go.mod": "module example.com/dependency\n\ngo 1.27.0\n",
				"dependency/dep.go": "package dependency\n",
				"goenv":             "GOFLAGS=-mod=readonly\n",
			})
			var stderr bytes.Buffer
			command := testOSCommand(root, io.Discard, &stderr)
			command.Env = append(command.Env,
				"GOENV="+filepath.Join(root, "goenv"), "GOPROXY=off", "GOTOOLCHAIN=local",
			)
			args := []string{"run"}
			switch mode {
			case "命令行":
				args = append(args, "-mod=mod")
			case "进程环境":
				command.Env = append(command.Env, "GOFLAGS=-mod=mod")
			case "关闭持久化配置":
				command.Env = append(command.Env, "GOENV=off")
			}
			if code := command.Run(args); code != 0 {
				t.Fatalf("显式覆盖失败: code=%d, %s", code, &stderr)
			}
			data, err := os.ReadFile(filepath.Join(root, "go.mod"))
			if err != nil || !bytes.Contains(data, []byte("require example.com/dependency")) {
				t.Fatalf("未按显式设置补齐依赖: %v\n%s", err, data)
			}
		})
	}
}
