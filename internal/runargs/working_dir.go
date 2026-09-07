package runargs

import (
	"fmt"
	"os"
	"path/filepath"

	"goark.dev/cli/internal/goargs"
)

// EffectiveWorkingDir 根据 Go 全局 -C 参数解析最终工作目录。
func EffectiveWorkingDir(base string, args []string) (string, error) {
	workingDir := BaseDir(base)
	directory := ""
	for entry := range goargs.Scan(args) {
		if entry.Name != "-C" {
			continue
		}
		if !entry.HasValue || entry.Value == "" {
			return "", fmt.Errorf("go 参数 -C 缺少目录")
		}
		directory = entry.Value
	}
	if directory == "" {
		return workingDir, nil
	}
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(workingDir, directory)
	}
	return filepath.Clean(directory), nil
}

// BaseDir 返回显式目录，空值时返回当前进程工作目录。
func BaseDir(dir string) string {
	if dir != "" {
		return filepath.Clean(dir)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return workingDir
}
