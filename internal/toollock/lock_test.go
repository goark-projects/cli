package toollock

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
)

func TestWriteAndRead_whenLockIsValid_shouldRoundTripDeterministically(t *testing.T) {
	root := t.TempDir()
	lock := File{
		Version:     CurrentVersion,
		BuildSHA256: strings.Repeat("a", 64),
		Tools: []Entry{
			{Name: "zeta", Type: buildspec.ToolTypeSystem, GOOS: "linux", GOARCH: "amd64", Path: "/usr/bin/zeta", SHA256: strings.Repeat("b", 64)},
			{Name: "alpha", Type: buildspec.ToolTypeGo, GOOS: "linux", GOARCH: "amd64", Package: "example.com/tools/alpha", Version: "v1.2.3", Module: "example.com/tools", ModuleVersion: "v1.2.3", ModuleSum: "h1:sum", Path: "/cache/alpha", SHA256: strings.Repeat("c", 64)},
		},
	}
	if err := Write(root, lock); err != nil {
		t.Fatalf("写入锁文件失败: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, buildspec.LockFileName))
	if err != nil {
		t.Fatalf("读取锁文件字节失败: %v", err)
	}
	if bytes.HasPrefix(first, []byte{0xef, 0xbb, 0xbf}) || bytes.ContainsRune(first, '\r') {
		t.Fatalf("锁文件不是 UTF-8 无 BOM、LF: %q", first)
	}
	if strings.Index(string(first), `name = 'alpha'`) > strings.Index(string(first), `name = 'zeta'`) {
		t.Fatalf("工具未稳定排序:\n%s", first)
	}

	loaded, err := Read(root)
	if err != nil {
		t.Fatalf("读取锁文件失败: %v", err)
	}
	lock.Tools[0], lock.Tools[1] = lock.Tools[1], lock.Tools[0]
	if !reflect.DeepEqual(loaded, lock) {
		t.Fatalf("锁文件 = %#v, want %#v", loaded, lock)
	}
	if err := Write(root, loaded); err != nil {
		t.Fatalf("重复写入锁文件失败: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(root, buildspec.LockFileName))
	if err != nil {
		t.Fatalf("读取第二次锁文件失败: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("锁文件输出不确定:\n%s\n---\n%s", first, second)
	}
}

func TestRead_whenLockIsInvalid_shouldReject(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unknown field", content: "version = 1\nbuild-sha256 = '" + strings.Repeat("a", 64) + "'\nunknown = true\n", want: "未知字段"},
		{name: "unsupported version", content: "version = 2\nbuild-sha256 = '" + strings.Repeat("a", 64) + "'\n", want: "version"},
		{name: "invalid digest", content: "version = 1\nbuild-sha256 = 'short'\n", want: "SHA-256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, buildspec.LockFileName), []byte(tt.content), 0o600); err != nil {
				t.Fatalf("写入测试锁文件失败: %v", err)
			}
			_, err := Read(root)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("错误 = %v, want contains %q", err, tt.want)
			}
		})
	}
}

func TestDigestFile_shouldReturnContentSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), buildspec.FileName)
	if err := os.WriteFile(path, []byte("version = 1\n"), 0o600); err != nil {
		t.Fatalf("写入描述文件失败: %v", err)
	}
	digest, err := DigestFile(path)
	if err != nil {
		t.Fatalf("计算摘要失败: %v", err)
	}
	if digest != "dbab12665d98aef021ba64953c61b0ed8a908cfb56a1c01e2fcb4b052b71a2a1" {
		t.Fatalf("摘要 = %s", digest)
	}
}

func TestFileVerifyBuild_whenDescriptionChanged_shouldReject(t *testing.T) {
	lock := File{BuildSHA256: strings.Repeat("a", 64)}
	if err := lock.VerifyBuild(strings.Repeat("b", 64)); err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("错误 = %v", err)
	}
}
