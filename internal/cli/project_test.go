package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectResolver_whenSingleCommandExists_shouldResolveModuleAndMain(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":              "module example.com/app\n\ngo 1.25\n",
		"internal/app/app.go": "package app\n",
		"cmd/server/main.go":  "package main\nfunc main() {}\n",
	})
	resolver := newTestProjectResolver(root)

	project, err := resolver.Resolve()
	if err != nil {
		t.Fatalf("发现项目失败: %v", err)
	}
	if project.Root != root || project.ModulePath != "example.com/app" {
		t.Fatalf("项目模型错误: %#v", project)
	}
	target, err := project.ResolveRunTarget(root)
	if err != nil {
		t.Fatalf("发现入口失败: %v", err)
	}
	if target != "./cmd/server" {
		t.Fatalf("入口 = %q", target)
	}
}

func TestProjectResolver_whenBuildFileMissing_shouldReject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("写入 go.mod 失败: %v", err)
	}
	_, err := newTestProjectResolver(root).Resolve()
	if err == nil || !strings.Contains(err.Error(), "goark.build") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestProjectResolver_whenConfiguredMainExists_shouldUseIt(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
		"goark.build":        "version = 1\n[project]\nmain = \"./cmd/admin\"\n",
		"cmd/admin/main.go":  "package main\nfunc main() {}\n",
		"cmd/worker/main.go": "package main\nfunc main() {}\n",
	})
	project, err := newTestProjectResolver(root).Resolve()
	if err != nil {
		t.Fatalf("发现项目失败: %v", err)
	}
	target, err := project.ResolveRunTarget(root)
	if err != nil || target != "./cmd/admin" {
		t.Fatalf("入口 = %q, err=%v", target, err)
	}
}

func TestProjectResolver_whenCurrentPackageIsMain_shouldPreferCurrentDirectory(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":            "module example.com/app\n\ngo 1.25\n",
		"main.go":           "package main\nfunc main() {}\n",
		"cmd/other/main.go": "package main\nfunc main() {}\n",
	})
	project, err := newTestProjectResolver(root).Resolve()
	if err != nil {
		t.Fatalf("发现项目失败: %v", err)
	}
	target, err := project.ResolveRunTarget(root)
	if err != nil {
		t.Fatalf("发现入口失败: %v", err)
	}
	if target != "." {
		t.Fatalf("入口 = %q", target)
	}
}

func TestProjectResolver_whenMultipleCommandsExist_shouldRequireExplicitTarget(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":             "module example.com/app\n\ngo 1.25\n",
		"cmd/admin/main.go":  "package main\nfunc main() {}\n",
		"cmd/worker/main.go": "package main\nfunc main() {}\n",
	})
	project, err := newTestProjectResolver(root).Resolve()
	if err != nil {
		t.Fatalf("发现项目失败: %v", err)
	}
	_, err = project.ResolveRunTarget(root)
	if err == nil || !strings.Contains(err.Error(), "./cmd/admin") || !strings.Contains(err.Error(), "./cmd/worker") {
		t.Fatalf("多入口错误不完整: %v", err)
	}
}

func TestProjectResolver_whenBuildTagsProvided_shouldUseMatchingFileSet(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod":      "module example.com/app\n\ngo 1.25\n",
		"app/base.go": "package app\n",
		"app/tagged.go": `//go:build special

package app

//goark:component
type TaggedComponent struct{}
`,
		"app/other.go": `//go:build other

package app

//goark:unknown
type OtherComponent struct{}
`,
	})
	resolver := newTestProjectResolver(root)
	resolver.BuildFlags = []string{"-tags=special"}
	project, err := resolver.Resolve()
	if err != nil {
		t.Fatalf("发现项目失败: %v", err)
	}
	results, err := generateProject(project, true)
	if err != nil {
		t.Fatalf("生成计划失败: %v", err)
	}
	if len(results) != 1 || results[0].Package != "example.com/app/app" {
		t.Fatalf("构建标签文件集错误: %#v", results)
	}
}

func TestProjectResolver_whenWorkspaceHasMultipleModules_shouldSelectContainingModule(t *testing.T) {
	workspace := t.TempDir()
	first := filepath.Join(workspace, "first")
	second := filepath.Join(workspace, "second")
	for _, item := range []struct {
		dir    string
		module string
	}{
		{dir: first, module: "example.com/first"},
		{dir: second, module: "example.com/second"},
	} {
		if err := os.MkdirAll(item.dir, 0o755); err != nil {
			t.Fatalf("创建模块目录失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(item.dir, "go.mod"), []byte("module "+item.module+"\n\ngo 1.25\n"), 0o644); err != nil {
			t.Fatalf("写入 go.mod 失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(item.dir, "goark.build"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatalf("写入 goark.build 失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(item.dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
			t.Fatalf("写入 main.go 失败: %v", err)
		}
	}
	goWork := "go 1.25\n\nuse (\n\t./first\n\t./second\n)\n"
	goWorkPath := filepath.Join(workspace, "go.work")
	if err := os.WriteFile(goWorkPath, []byte(goWork), 0o644); err != nil {
		t.Fatalf("写入 go.work 失败: %v", err)
	}
	resolver := projectResolver{
		Dir:    second,
		Env:    append(os.Environ(), "GOWORK="+goWorkPath, "GOFLAGS="),
		Runner: osProcessRunner{},
		Err:    io.Discard,
	}

	project, err := resolver.Resolve()
	if err != nil {
		t.Fatalf("发现工作区项目失败: %v", err)
	}
	if project.Root != second || project.ModulePath != "example.com/second" {
		t.Fatalf("选择了错误模块: %#v", project)
	}
}

func newTestProjectResolver(dir string) projectResolver {
	return projectResolver{
		Dir:    dir,
		Env:    append(os.Environ(), "GOWORK=off", "GOFLAGS="),
		Runner: osProcessRunner{},
		Err:    io.Discard,
	}
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
