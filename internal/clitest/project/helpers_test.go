package clitest

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/cli"
	"goark.dev/cli/internal/processrun"
)

type Command = cli.Command

type ProcessRequest = cli.ProcessRequest

type recordingProcessRunner struct {
	requests []ProcessRequest
	err      error
}

func (r *recordingProcessRunner) Run(request ProcessRequest) error {
	r.requests = append(r.requests, request)
	return r.err
}

func writeTestModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if _, exists := files["goark.build"]; !exists {
		files["goark.build"] = "version = 1\n"
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("写入 %s 失败: %v", name, err)
		}
	}
	return root
}

func assertOrderedFragments(t *testing.T, value string, fragments []string) {
	t.Helper()
	position := 0
	for _, fragment := range fragments {
		index := strings.Index(value[position:], fragment)
		if index < 0 {
			t.Fatalf("输出缺少有序片段 %q: %q", fragment, value)
		}
		position += index + len(fragment)
	}
}

func testOSCommand(dir string, stdout io.Writer, stderr io.Writer) Command {
	return Command{
		Dir:    dir,
		Env:    append(os.Environ(), "GOWORK=off", "GOFLAGS="),
		Out:    stdout,
		Err:    stderr,
		Runner: processrun.OSRunner{},
	}
}
