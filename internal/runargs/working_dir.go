package runargs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EffectiveWorkingDir 根据 Go 全局 -C 参数解析最终工作目录。
func EffectiveWorkingDir(base string, args []string) (string, error) {
	workingDir := BaseDir(base)
	directory := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		var value string
		switch {
		case arg == "-C":
			if index+1 >= len(args) {
				return "", fmt.Errorf("go 参数 -C 缺少目录")
			}
			value = args[index+1]
		case strings.HasPrefix(arg, "-C="):
			value = strings.TrimPrefix(arg, "-C=")
		}
		if value != "" {
			directory = value
		}
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
