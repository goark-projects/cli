package projectlock

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquire_whenContended_shouldWaitForContextAndRelease(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatalf("获取首个项目锁失败: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Acquire(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("竞争锁错误 = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("释放项目锁失败: %v", err)
	}
	second, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatalf("重新获取项目锁失败: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatalf("释放第二个项目锁失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".goark", "locks", "project.lock")); err != nil {
		t.Fatalf("项目锁文件不存在: %v", err)
	}
}
