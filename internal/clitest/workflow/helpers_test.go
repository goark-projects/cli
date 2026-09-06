package clitest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/cli"
)

type Command = cli.Command

type ProcessRequest = cli.ProcessRequest

type recordingProcessRunner struct {
	requests []ProcessRequest
	err      error
}

type processExitError struct {
	code int
}

func (e processExitError) Error() string {
	return "进程退出"
}

func (e processExitError) ExitCode() int {
	return e.code
}

type infoReport struct {
	Profile    string          `json:"profile"`
	Generators []infoGenerator `json:"generators"`
	Plans      []infoPlan      `json:"plans"`
}

type infoGenerator struct {
	Packages int `json:"packages"`
}

type infoPlan struct {
	GoArguments []string          `json:"goArguments"`
	Environment map[string]string `json:"environment"`
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

func canonicalTestPath(t *testing.T, value string) string {
	t.Helper()
	abs, err := filepath.Abs(value)
	if err != nil {
		t.Fatalf("解析绝对路径失败: %v", err)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatalf("解析路径符号链接失败: %v", err)
	}
	return filepath.Clean(canonical)
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
