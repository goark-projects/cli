//go:build !windows

package projectlock

import "path/filepath"

// NormalizeRoot 规范化项目锁使用的路径身份。
func NormalizeRoot(root string) string {
	return filepath.Clean(root)
}
