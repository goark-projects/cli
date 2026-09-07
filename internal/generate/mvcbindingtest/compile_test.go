package generate_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func assertGeneratedSourceParses(t *testing.T, generated []byte) {
	t.Helper()
	if _, err := parser.ParseFile(
		token.NewFileSet(),
		"zz_goark_app_gen.go",
		generated,
		parser.ParseComments,
	); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
}

func assertGeneratedPackageBuilds(t *testing.T, dir string, generated []byte) {
	t.Helper()
	t.Run("本地集成编译", func(t *testing.T) {
		if os.Getenv("GOARK_INTEGRATION_TESTS") != "1" {
			t.Skip("设置 GOARK_INTEGRATION_TESTS=1 后执行本地兄弟仓库集成编译")
		}
		path := filepath.Join(dir, "zz_goark_app_gen.go")
		if err := os.WriteFile(path, generated, 0o644); err != nil {
			t.Fatalf("write generated source failed: %v", err)
		}
		goarkRoot := filepath.ToSlash(
			filepath.Clean(filepath.Join(repositoryRoot(t), "..", "goark")),
		)
		mod := fmt.Sprintf(`module example.com/goark-generated-test

go 1.26.0

require goark.dev/goark v0.0.1

replace goark.dev/goark => %s
`, goarkRoot)
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
			t.Fatalf("write generated test module failed: %v", err)
		}
		cmd := exec.Command("go", "test", "-mod=mod", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("generated package should compile: %v\n%s", err, string(output))
		}
	})
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
