//go:build windows

package projectlock

import (
	"path/filepath"
	"strings"
)

// NormalizeRoot 规范化项目锁使用的路径身份。
func NormalizeRoot(root string) string {
	return strings.ToLower(filepath.Clean(root))
}
