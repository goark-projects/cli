package application_test

import (
	"os"
	"path/filepath"
	"testing"

	"goark.dev/cli/internal/generate"
)

func writeSource(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "application.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("写入测试源码失败: %v", err)
	}
	return dir
}

func generatedFile(t *testing.T, files []generate.AnnotationFile, name string) []byte {
	t.Helper()
	for _, file := range files {
		if file.Name == name {
			return file.Source
		}
	}
	t.Fatalf("未生成 %s: %#v", name, files)
	return nil
}
