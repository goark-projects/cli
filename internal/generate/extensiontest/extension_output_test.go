package generate_test

import (
	"os"
	"path/filepath"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestExtensionOutputIdentityRejectsAmbiguity(t *testing.T) {
	for _, name := range []string{"core", " CORE ", "../escape",
		"x/y", "x\\y"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "app.go")
			if err := os.WriteFile(path, []byte("package app\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := generate.GenerateAnnotationFiles(generate.AnnotationScanSpec{
				Dir: dir, Extensions: []generate.AnnotationExtension{{Name: name}},
			})
			if err == nil {
				t.Fatalf("应拒绝冲突或非法扩展名 %q", name)
			}
		})
	}
}
