package clitest

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCommand_whenGoCommandFails_shouldStillRunFinally(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.26.0\n",
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
		t.Fatalf(
			"构建必须失败, stdout=%s stderr=%s",
			stdout.String(),
			stderr.String(),
		)
	}
	if !strings.Contains(stdout.String(), "go version") {
		t.Fatalf("finally 未执行: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestCommand_whenFinallyTaskAlreadyRan_shouldRunItAgain(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.26.0\n",
		"goark.build": `version = 1
[commands.build]
before = ["cleanup"]
finally = ["cleanup"]

[tasks.cleanup]
type = "go"
args = ["version"]
`,
	})
	var stderr bytes.Buffer
	command := testOSCommand(root, io.Discard, &stderr)
	args := []string{"build", "--goark-dry-run", "./..."}
	if code := command.Run(args); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	count := strings.Count(stderr.String(), "would run task cleanup")
	if count != 2 {
		t.Fatalf(
			"finally 必须独立执行同名任务，执行次数 = %d:\n%s",
			count,
			stderr.String(),
		)
	}
}
