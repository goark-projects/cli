package projectlock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Lock 表示当前进程持有的项目级跨进程锁。
type Lock struct {
	file *flock.Flock
}

// Acquire 等待获取项目级跨进程锁，等待过程支持取消。
func Acquire(ctx context.Context, root string) (*Lock, error) {
	directory := filepath.Join(root, ".goark", "locks")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("创建项目锁目录失败: %w", err)
	}
	file := flock.New(filepath.Join(directory, "project.lock"))
	locked, err := file.TryLockContext(ctx, 25*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("获取项目锁失败: %w", err)
	}
	if !locked {
		return nil, ctx.Err()
	}
	return &Lock{file: file}, nil
}

// Release 释放项目级跨进程锁。
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	if err := l.file.Unlock(); err != nil {
		return fmt.Errorf("释放项目锁失败: %w", err)
	}
	l.file = nil
	return nil
}
