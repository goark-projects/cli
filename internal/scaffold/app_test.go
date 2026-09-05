package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
)

func TestCreateApp_whenWebEnabled_shouldWriteBootWebSkeleton(t *testing.T) {
	dir := t.TempDir()

	files, err := CreateApp(AppSpec{
		Dir:        dir,
		ModulePath: "example.com/admin",
		Web:        true,
	})
	if err != nil {
		t.Fatalf("create app failed: %v", err)
	}

	expected := []string{
		".gitignore",
		"README.md",
		"go.mod",
		"goark.build",
		"resource/app.yml",
		"resource/static/index.html",
		"cmd/server/main.go",
		"internal/app/configuration.go",
	}
	if len(files) != len(expected) {
		t.Fatalf("file count = %d, want %d", len(files), len(expected))
	}
	for _, path := range expected {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}
	assertFileContains(t, filepath.Join(dir, "go.mod"), "module example.com/admin")
	assertFileContains(t, filepath.Join(dir, "goark.build"), "name = \"admin\"")
	assertFileContains(t, filepath.Join(dir, "goark.build"), "main = \"./cmd/server\"")
	assertFileContains(t, filepath.Join(dir, ".gitignore"), "/.goark/")
	assertFileContains(t, filepath.Join(dir, "README.md"), "goark run")
	assertFileContains(t, filepath.Join(dir, "go.mod"), "goark.dev/gbc-web v0.0.0")
	assertFileContains(t, filepath.Join(dir, "resource/app.yml"), "max-response-bytes")
	assertFileContains(t, filepath.Join(dir, "resource/static/index.html"), "Goark Boot Web application is running.")
	assertFileContains(t, filepath.Join(dir, "cmd/server/main.go"), `app "example.com/admin/internal/app"`)
	assertFileContains(t, filepath.Join(dir, "internal/app/configuration.go"), `mvc.GET("/healthz"`)
	assertFileContains(t, filepath.Join(dir, "internal/app/configuration.go"), `gbcweb.RegisterHTTPClientBuilderCustomizer`)
	if _, err := buildspec.LoadFile(filepath.Join(dir, buildspec.FileName)); err != nil {
		t.Fatalf("generated goark.build is invalid: %v", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(dir, file.Path))
		if err != nil {
			t.Fatalf("read generated file %s failed: %v", file.Path, err)
		}
		if strings.HasPrefix(string(data), "\ufeff") || strings.ContainsRune(string(data), '\r') {
			t.Fatalf("generated file %s is not UTF-8 without BOM and LF", file.Path)
		}
	}

	writeLocalReplaces(t, dir)
	assertGeneratedAppBuilds(t, dir)
}

func TestCreateApp_whenTargetExistsWithoutForce_shouldReturnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module existing\n"), 0o644); err != nil {
		t.Fatalf("write existing go.mod failed: %v", err)
	}

	_, err := CreateApp(AppSpec{
		Dir:        dir,
		ModulePath: "example.com/admin",
		Web:        true,
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing file error, got %v", err)
	}
	for _, path := range []string{".gitignore", "README.md"} {
		if _, statErr := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(statErr) {
			t.Fatalf("preflight failure must not leave partial file %s: %v", path, statErr)
		}
	}
}

func TestCreateApp_whenModuleMissing_shouldReturnError(t *testing.T) {
	_, err := CreateApp(AppSpec{Dir: t.TempDir(), Web: true})
	if err == nil || !strings.Contains(err.Error(), "module path is required") {
		t.Fatalf("expected module path error, got %v", err)
	}
}

func assertFileContains(t *testing.T, path string, fragment string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s failed: %v", path, err)
	}
	if !strings.Contains(string(data), fragment) {
		t.Fatalf("%s missing %q:\n%s", path, fragment, string(data))
	}
}

func writeLocalReplaces(t *testing.T, dir string) {
	t.Helper()

	root := filepath.Clean(filepath.Join(projectRoot(t), ".."))
	mod := `module example.com/admin

go 1.25

require (
	goark.dev/arkarta v0.0.0
	goark.dev/arkhos v0.0.0
	goark.dev/boot v0.0.0
	goark.dev/gbc-arkhos v0.0.0
	goark.dev/gbc-web v0.0.0
	goark.dev/goark v0.0.0
)

replace goark.dev/arkarta => ` + filepath.ToSlash(filepath.Join(root, "arkarta")) + `

replace goark.dev/arkhos => ` + filepath.ToSlash(filepath.Join(root, "arkhos")) + `

replace goark.dev/boot => ` + filepath.ToSlash(filepath.Join(root, "goark-boot")) + `

replace goark.dev/gbc-arkhos => ` + filepath.ToSlash(filepath.Join(root, "goark-boot-contrib-arkhos")) + `

replace goark.dev/gbc-web => ` + filepath.ToSlash(filepath.Join(root, "goark-boot-contrib-web")) + `

replace goark.dev/goark => ` + filepath.ToSlash(filepath.Join(root, "goark")) + `
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatalf("write local go.mod failed: %v", err)
	}
}

func assertGeneratedAppBuilds(t *testing.T, dir string) {
	t.Helper()

	for _, args := range [][]string{{"mod", "tidy"}, {"test", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("generated app command go %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
		}
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
