package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCommand_whenDoctorChecksHealthyProject_shouldPass(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
		"cmd/server/main.go": "package main\nfunc main() {}\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := testOSCommand(root, &stdout, &stderr)
	command.TrustDir = t.TempDir()
	command.ToolCacheDir = t.TempDir()
	if code := command.Run([]string{"doctor"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	for _, fragment := range []string{"PASS goark.build", "PASS task graph", "PASS go toolchain"} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("诊断缺少 %q:\n%s", fragment, stdout.String())
		}
	}
}

func TestCommand_whenDoctorFindsUnavailableTool_shouldFailWithoutInstalling(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.25\n",
		"goark.build": `version = 1
[tools.missing]
type = "system"
command = "goark-command-that-does-not-exist"
install = "manual"
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := testOSCommand(root, &stdout, &stderr)
	command.TrustDir = t.TempDir()
	command.ToolCacheDir = t.TempDir()
	if code := command.Run([]string{"doctor"}); code != 1 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "FAIL tool missing") {
		t.Fatalf("工具失败诊断缺失:\n%s", stdout.String())
	}
}

func TestCommand_whenDoctorHasArguments_shouldReturnUsageError(t *testing.T) {
	var stderr bytes.Buffer
	command := Command{Out: io.Discard, Err: &stderr}
	if code := command.Run([]string{"doctor", "--json"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
}
