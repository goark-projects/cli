package projecttrust

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStoreTrust_whenProjectAndDigestMatch_shouldPersistAndVerify(t *testing.T) {
	root := t.TempDir()
	store := Store{Dir: t.TempDir()}
	digest := strings.Repeat("a", 64)
	if err := store.Trust(root, digest); err != nil {
		t.Fatalf("写入信任记录失败: %v", err)
	}
	if err := store.Verify(root, digest); err != nil {
		t.Fatalf("验证信任记录失败: %v", err)
	}
	path, err := store.recordPath(root)
	if err != nil {
		t.Fatalf("解析信任路径失败: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取信任记录失败: %v", err)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || bytes.ContainsRune(data, '\r') {
		t.Fatalf("信任记录不是 UTF-8 无 BOM、LF: %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("读取信任记录权限失败: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("信任记录权限过宽: %o", info.Mode().Perm())
	}
}

func TestStoreVerify_whenBuildDigestChanges_shouldReject(t *testing.T) {
	root := t.TempDir()
	store := Store{Dir: t.TempDir()}
	if err := store.Trust(root, strings.Repeat("a", 64)); err != nil {
		t.Fatalf("写入信任记录失败: %v", err)
	}
	err := store.Verify(root, strings.Repeat("b", 64))
	if err == nil || !strings.Contains(err.Error(), "不受信任") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestStoreVerify_whenRecordBelongsToAnotherRoot_shouldReject(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	store := Store{Dir: t.TempDir()}
	digest := strings.Repeat("a", 64)
	if err := store.Trust(root, digest); err != nil {
		t.Fatalf("写入信任记录失败: %v", err)
	}
	source, _ := store.recordPath(root)
	target, _ := store.recordPath(other)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	data, _ := os.ReadFile(source)
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatalf("复制信任记录失败: %v", err)
	}
	if err := store.Verify(other, digest); err == nil {
		t.Fatal("其他项目不应复用信任记录")
	}
}
