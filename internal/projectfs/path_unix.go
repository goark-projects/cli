//go:build !windows

package projectfs

import "path/filepath"

// SamePath 按当前操作系统语义比较两个清理后的路径。
func SamePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
