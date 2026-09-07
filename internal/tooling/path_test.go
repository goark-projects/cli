package tooling

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalExecutable_whenFileIsExecutable_shouldReturnUsablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(path, []byte("tool\n"), 0o755); err != nil {
		t.Fatalf("写入测试工具失败: %v", err)
	}

	actual, err := canonicalExecutable(path)
	if err != nil {
		t.Fatalf("解析可执行文件失败: %v", err)
	}
	if !executableFile(actual) {
		t.Fatalf("返回路径不可执行: %s", actual)
	}
}

func TestCanonicalExecutable_whenFileDoesNotExist_shouldReject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")

	if _, err := canonicalExecutable(path); err == nil {
		t.Fatal("不存在的工具应被拒绝")
	}
}

func TestCanonicalExecutable_whenLinkEvaluationFails_shouldUseValidatedAbsolutePath(
	t *testing.T,
) {
	path := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(path, []byte("tool\n"), 0o755); err != nil {
		t.Fatalf("写入测试工具失败: %v", err)
	}

	actual, err := canonicalExecutablePath(path, func(string) (string, error) {
		return "", errors.New("无法展开目录链接")
	})
	if err != nil {
		t.Fatalf("合法工具不应因目录链接无法展开而失败: %v", err)
	}
	expected, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("解析预期绝对路径失败: %v", err)
	}
	if actual != filepath.Clean(expected) {
		t.Fatalf("路径 = %q, 期望 %q", actual, filepath.Clean(expected))
	}
}
