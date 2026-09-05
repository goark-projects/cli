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

func TestCommand_whenSyncRequested_shouldCreateVerifiableLockAndTrust(t *testing.T) {
	root := toolCommandProject(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := testToolCommand(t, root, &stdout, &stderr)
	if code := command.Run([]string{"sync"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "goark.build.lock")); err != nil {
		t.Fatalf("锁文件不存在: %v", err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "goark.build.lock"))
	if code := command.Run([]string{"sync", "--locked"}); code != 0 {
		t.Fatalf("锁定验证退出码 = %d, stderr=%s", code, stderr.String())
	}
	after, _ := os.ReadFile(filepath.Join(root, "goark.build.lock"))
	if !bytes.Equal(before, after) {
		t.Fatal("--locked 修改了锁文件")
	}
}

func TestCommand_whenToolsJSONRequested_shouldReportStableStatuses(t *testing.T) {
	root := toolCommandProject(t)
	command := testToolCommand(t, root, io.Discard, io.Discard)
	if code := command.Run([]string{"sync"}); code != 0 {
		t.Fatalf("同步退出码 = %d", code)
	}
	var stdout bytes.Buffer
	command.Out = &stdout
	if code := command.Run([]string{"tools", "--json"}); code != 0 {
		t.Fatalf("工具查询退出码 = %d", code)
	}
	var statuses []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &statuses); err != nil {
		t.Fatalf("JSON 无效: %v\n%s", err, stdout.String())
	}
	if len(statuses) != 1 || statuses[0].Name != "go" || statuses[0].Status != "ready" {
		t.Fatalf("工具状态错误: %#v", statuses)
	}
}

func TestCommand_whenToolVerifyRequestedAfterDrift_shouldFail(t *testing.T) {
	root := toolCommandProject(t)
	command := testToolCommand(t, root, io.Discard, io.Discard)
	if code := command.Run([]string{"sync"}); code != 0 {
		t.Fatalf("同步退出码 = %d", code)
	}
	path := filepath.Join(root, "goark.build")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(data, []byte("# changed\n")...), 0o644); err != nil {
		t.Fatalf("修改描述文件失败: %v", err)
	}
	var stderr bytes.Buffer
	command.Err = &stderr
	if code := command.Run([]string{"tool", "verify"}); code == 0 {
		t.Fatal("锁文件漂移后验证必须失败")
	}
	if !strings.Contains(stderr.String(), "摘要不一致") {
		t.Fatalf("错误信息不完整: %q", stderr.String())
	}
}

func TestCommand_whenToolInstallUnknownNameRequested_shouldReturnUsageError(t *testing.T) {
	root := toolCommandProject(t)
	var stderr bytes.Buffer
	command := testToolCommand(t, root, io.Discard, &stderr)
	if code := command.Run([]string{"tool", "install", "missing"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
	if !strings.Contains(stderr.String(), "不存在") {
		t.Fatalf("错误信息不完整: %q", stderr.String())
	}
}

func toolCommandProject(t *testing.T) string {
	t.Helper()
	return writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"goark.build": "version = 1\n[tools.go]\ntype = \"system\"\ncommand = \"go\"\ninstall = \"manual\"\n",
	})
}

func testToolCommand(t *testing.T, root string, stdout io.Writer, stderr io.Writer) Command {
	t.Helper()
	return Command{
		Dir: root, Env: append(os.Environ(), "GOWORK=off"), Out: stdout, Err: stderr,
		Runner: osProcessRunner{}, TrustDir: t.TempDir(), ToolCacheDir: t.TempDir(),
	}
}
