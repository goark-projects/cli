package quality_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

const maximumGoSourceLines = 360

func TestGoSourceFiles_whenLineLimitExceeded_shouldReject(t *testing.T) {
	forEachGoSource(t, func(path string, data []byte) {
		lines := bytes.Count(data, []byte{'\n'})
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lines++
		}
		if lines > maximumGoSourceLines {
			t.Errorf("%s 包含 %d 行，超过 %d 行限制", path, lines, maximumGoSourceLines)
		}
	})
}

func TestGoSourceFiles_whenEncodingIsNotUTF8LF_shouldReject(t *testing.T) {
	forEachGoSource(t, func(path string, data []byte) {
		if !utf8.Valid(data) {
			t.Errorf("%s 不是有效 UTF-8", path)
		}
		if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
			t.Errorf("%s 包含 UTF-8 BOM", path)
		}
		if bytes.ContainsRune(data, '\r') {
			t.Errorf("%s 包含非 LF 换行", path)
		}
	})
}

func forEachGoSource(t *testing.T, check func(string, []byte)) {
	t.Helper()
	root := repositoryRoot(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		check(filepath.ToSlash(relative), data)
		return nil
	})
	if err != nil {
		t.Fatalf("扫描 Go 源文件失败: %v", err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("解析质量测试路径失败")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
