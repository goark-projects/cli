package buildspec

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var tomlExamplePattern = regexp.MustCompile("(?s)```toml\\n(.*?)\\n```")

func TestDocumentation_whenCompleteBuildExamplesDeclared_shouldParse(t *testing.T) {
	root := documentationRoot(t)
	paths := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "README.zh-CN.md"),
	}
	if err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("扫描文档失败: %v", err)
	}

	validated := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取文档 %s 失败: %v", path, err)
		}
		for index, match := range tomlExamplePattern.FindAllSubmatch(data, -1) {
			if !strings.Contains(string(match[1]), "version = 1") {
				continue
			}
			buildPath := filepath.Join(t.TempDir(), FileName)
			if err := os.WriteFile(buildPath, append(match[1], '\n'), 0o600); err != nil {
				t.Fatalf("写入文档示例失败: %v", err)
			}
			if _, err := LoadFile(buildPath); err != nil {
				t.Errorf("文档 %s 的 TOML 示例 %d 无效: %v", filepath.Base(path), index+1, err)
			}
			validated++
		}
	}
	if validated == 0 {
		t.Fatal("文档中未找到完整 goark.build 示例")
	}
}

func documentationRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位文档测试文件")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
