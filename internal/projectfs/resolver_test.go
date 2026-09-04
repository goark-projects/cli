package projectfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolverResolve_whenPathIsInsideProject_shouldReturnCanonicalPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "app")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("创建项目目录失败: %v", err)
	}
	resolved, err := New(root).Resolve("./internal/app", MustExist)
	if err != nil {
		t.Fatalf("解析项目路径失败: %v", err)
	}
	if resolved != path {
		t.Fatalf("路径 = %q, want %q", resolved, path)
	}
}

func TestResolverResolve_whenMissingOutputIsInsideProject_shouldResolveExistingParent(t *testing.T) {
	root := t.TempDir()
	resolved, err := New(root).Resolve("build/output/app", AllowMissing)
	if err != nil {
		t.Fatalf("解析输出路径失败: %v", err)
	}
	want := filepath.Join(root, "build", "output", "app")
	if resolved != want {
		t.Fatalf("路径 = %q, want %q", resolved, want)
	}
}

func TestResolverResolve_whenPathEscapesProject_shouldReject(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"../outside", filepath.Join(root, "..", "outside")} {
		_, err := New(root).Resolve(value, AllowMissing)
		if err == nil || !strings.Contains(err.Error(), "项目根目录") {
			t.Fatalf("路径 %q 的错误 = %v", value, err)
		}
	}
}

func TestResolverResolve_whenSymlinkEscapesProject_shouldReject(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建项目目录失败: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("创建外部目录失败: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("当前环境无法创建符号链接: %v", err)
	}
	_, err := New(root).Resolve("link/file.txt", AllowMissing)
	if err == nil || !strings.Contains(err.Error(), "符号链接") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestResolverResolvePattern_whenStaticPrefixEscapesProject_shouldReject(t *testing.T) {
	_, err := New(t.TempDir()).ResolvePattern("../outside/**/*.go")
	if err == nil || !strings.Contains(err.Error(), "项目根目录") {
		t.Fatalf("错误 = %v", err)
	}
}
