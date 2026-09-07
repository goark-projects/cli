package quality_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

const maximumGoSourceLines = 360
const maximumGoSourceLineLength = 100
const maximumGoPackageFiles = 20

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

func TestGoSourceFiles_whenLineIsTooLong_shouldReject(t *testing.T) {
	forEachGoSource(t, func(path string, data []byte) {
		for index, line := range bytes.Split(data, []byte{'\n'}) {
			length := utf8.RuneCount(line)
			if length > maximumGoSourceLineLength {
				t.Errorf(
					"%s:%d 包含 %d 个字符，超过 %d 字符限制",
					path,
					index+1,
					length,
					maximumGoSourceLineLength,
				)
			}
		}
	})
}

func TestGoPackages_whenDirectFileLimitExceeded_shouldReject(t *testing.T) {
	counts := make(map[string]int)
	forEachGoSource(t, func(path string, _ []byte) {
		counts[filepath.Dir(path)]++
	})
	directories := make([]string, 0, len(counts))
	for directory := range counts {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	for _, directory := range directories {
		if counts[directory] > maximumGoPackageFiles {
			t.Errorf(
				"%s 直属包含 %d 个 Go 文件，超过 %d 个文件限制",
				directory, counts[directory], maximumGoPackageFiles,
			)
		}
	}
}

func forEachGoSource(t *testing.T, check func(string, []byte)) {
	t.Helper()
	root := repositoryRoot(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "gen" {
				return filepath.SkipDir
			}
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
