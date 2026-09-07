package projectfs

import (
	"os"
	"path/filepath"
)

// WorkspaceActive 判断显式或向上发现的 Go 工作区，异常时保留 Go 的默认决策。
func WorkspaceActive(root, setting string) bool {
	if setting == "off" {
		return false
	}
	if setting != "" && setting != "auto" {
		return true
	}
	for {
		info, err := os.Stat(filepath.Join(root, "go.work"))
		if err == nil && !info.IsDir() || err != nil && !os.IsNotExist(err) {
			return true
		}
		parent := filepath.Dir(root)
		if parent == root {
			return false
		}
		root = parent
	}
}
